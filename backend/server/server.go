package server

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ron96g/yak8sui/backend/auth"
	"github.com/ron96g/yak8sui/backend/config"
	"github.com/ron96g/yak8sui/backend/resources"
	"golang.org/x/oauth2"
)

const (
	loginCookie   = "yak8sui_login"
	sessionCookie = "yak8sui_session"
	fieldID       = "id"
	fieldName     = "name"
	fieldError    = "error"
)

type oidcClient interface {
	AuthorizationURL(state, challenge, nonce string) string
	Exchange(context.Context, string, string, string) (auth.Identity, error)
}
type loginAttempt struct {
	verifier, nonce string
	expires         time.Time
}

type Server struct {
	cfg           config.Config
	oidc          oidcClient
	sessions      *auth.Sessions
	mux           *http.ServeMux
	attemptsMu    sync.Mutex
	attempts      map[string]loginAttempt
	resources     *resources.Projection
	catalog       *resources.Catalog
	relationships resources.RelationshipResolver
}

func New(cfg *config.Config, oidcClient oidcClient, sessions *auth.Sessions, projection *resources.Projection, catalog *resources.Catalog) *Server {
	s := &Server{cfg: *cfg, oidc: oidcClient, sessions: sessions, mux: http.NewServeMux(), attempts: map[string]loginAttempt{}}
	s.resources = projection
	s.catalog = catalog
	if projection != nil {
		s.relationships = resources.NewOwnerReferencesResolver(projection)
	}
	s.routes()
	return s
}
func (s *Server) Handler() http.Handler { return securityHeaders(s.mux) }

func (s *Server) routes() {
	s.mux.HandleFunc("GET /auth/login", s.login)
	s.mux.HandleFunc("GET /auth/callback", s.callback)
	s.mux.HandleFunc("POST /api/logout", s.logout)
	s.mux.HandleFunc("GET /api/session", s.requireSession(s.sessionInfo))
	s.mux.HandleFunc("GET /api/tenants/{tenant}/context", s.requireTenant(s.tenantContext))
	if s.resources != nil && s.catalog != nil {
		s.mux.HandleFunc("GET /api/v1/configuration", s.requireSession(s.configuration))
		s.mux.HandleFunc("GET /api/v1/tenants/{tenant}/resource-streams/{group}/{version}/{resource}", s.requireTenant(s.resourceStream))
		s.mux.HandleFunc("GET /api/v1/tenants/{tenant}/resources/{group}/{version}/{resource}", s.requireTenant(s.resourceList))
		s.mux.HandleFunc("GET /api/v1/tenants/{tenant}/resources/{group}/{version}/{resource}/{uid}", s.requireTenant(s.resourceDetail))
		s.mux.HandleFunc("GET /api/v1/tenants/{tenant}/relationships/{group}/{version}/{resource}/{uid}", s.requireTenant(s.relationshipNeighborhood))
		s.mux.HandleFunc("GET /api/v1/tenants/{tenant}/relationship-streams/{group}/{version}/{resource}/{uid}", s.requireTenant(s.relationshipStream))
	}
	if _, err := os.Stat(s.cfg.Server.WebDirectory); err == nil {
		s.mux.Handle("/", spaHandler(s.cfg.Server.WebDirectory))
	} else {
		slog.Warn("frontend build directory unavailable", "directory", s.cfg.Server.WebDirectory)
		s.mux.HandleFunc("GET /", func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "frontend is not built; run npm run build", http.StatusServiceUnavailable)
		})
	}
}

func (s *Server) relationshipNeighborhood(w http.ResponseWriter, r *http.Request, tenant config.TenantConfig) {
	resourceType, ok := s.requestedType(r)
	if !ok {
		writeError(w, http.StatusNotFound, "configured resource type not found")
		return
	}
	result, err := s.relationships.Resolve(r.Context(), tenant.ID, &resourceType, r.PathValue("uid"), s.cfg.Observation.Relationships, s.catalog)
	if s.writeRelationshipError(w, r, &resourceType, err) {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) relationshipStream(w http.ResponseWriter, r *http.Request, tenant config.TenantConfig) {
	resourceType, ok := s.requestedType(r)
	if !ok {
		writeError(w, http.StatusNotFound, "configured resource type not found")
		return
	}
	session, ok := sessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusInternalServerError, "session context unavailable")
		return
	}
	resolve := func() (resources.Neighborhood, error) {
		return s.relationships.Resolve(r.Context(), tenant.ID, &resourceType, r.PathValue("uid"), s.cfg.Observation.Relationships, s.catalog)
	}
	result, subscription, err := s.resources.RelationshipSnapshotAndSubscribe(s.relationshipTypeKeys(&resourceType), resolve)
	if s.writeRelationshipError(w, r, &resourceType, err) {
		return
	}
	defer subscription.Close()
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is unavailable")
		return
	}
	setSSEHeaders(w)
	if !writeSSEEvent(w, "snapshot", result) {
		return
	}
	flusher.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-session.Done():
			return
		case _, open := <-subscription.Updates:
			if !open {
				return
			}
			result, err = resolve()
			if errors.Is(err, resources.ErrNotSynchronized) {
				status, statusErr := s.resources.ObservationStatus(r.Context(), resourceType.Key())
				if statusErr != nil {
					return
				}
				_ = writeSSEEvent(w, "observation", status)
				flusher.Flush()
				return
			}
			if errors.Is(err, sql.ErrNoRows) {
				_ = writeSSEEvent(w, "revoked", map[string]string{"reason": "resource is no longer visible"})
				flusher.Flush()
				return
			}
			if err != nil || !writeSSEEvent(w, "snapshot", result) {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *Server) relationshipTypeKeys(center *resources.Type) []string {
	keys := map[string]bool{center.Key(): true}
	for _, rule := range s.cfg.Observation.Relationships {
		for _, endpoint := range []config.RelationshipType{rule.Parent, rule.Child} {
			if item, ok := s.catalog.FindByGroupKind(endpoint.Group, endpoint.Kind); ok {
				keys[item.Key()] = true
			}
		}
	}
	result := make([]string, 0, len(keys))
	for key := range keys {
		result = append(result, key)
	}
	return result
}

func (s *Server) writeRelationshipError(w http.ResponseWriter, r *http.Request, resourceType *resources.Type, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, resources.ErrNotSynchronized) {
		s.writeObservationError(w, r, resourceType.Key())
		return true
	}
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "resource not found")
		return true
	}
	slog.Error("resolve ownership neighborhood", "error", err)
	writeError(w, http.StatusInternalServerError, "could not resolve ownership neighborhood")
	return true
}

func (s *Server) resourceStream(w http.ResponseWriter, r *http.Request, tenant config.TenantConfig) {
	resourceType, ok := s.requestedType(r)
	if !ok {
		writeError(w, http.StatusNotFound, "configured resource type not found")
		return
	}
	if !resourceType.Available {
		s.writeObservationStream(w, r, &resourceType)
		return
	}
	session, ok := sessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusInternalServerError, "session context unavailable")
		return
	}
	query, err := resourceQuery(r, &resourceType)
	if err != nil {
		writeQueryError(w, err)
		return
	}
	page, subscription, err := s.resources.SnapshotAndSubscribe(r.Context(), tenant.ID, resourceType.Key(), &query)
	if errors.Is(err, resources.ErrNotSynchronized) {
		s.writeObservationStream(w, r, &resourceType)
		return
	}
	if err != nil {
		slog.Error("start resource stream", "error", err)
		writeError(w, http.StatusInternalServerError, "could not stream resources")
		return
	}
	defer subscription.Close()
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is unavailable")
		return
	}
	setSSEHeaders(w)
	if !writeSSESnapshot(w, page) {
		return
	}
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-session.Done():
			return
		case _, open := <-subscription.Updates:
			if !open {
				return
			}
			query.ClampPage = true
			page, err = s.resources.List(r.Context(), tenant.ID, resourceType.Key(), &query)
			if errors.Is(err, resources.ErrNotSynchronized) {
				status, statusErr := s.resources.ObservationStatus(r.Context(), resourceType.Key())
				if statusErr == nil {
					if !writeSSEEvent(w, "observation", status) {
						return
					}
					flusher.Flush()
				}
				return
			}
			query.Page = page.Page
			if err != nil {
				return
			}
			if !writeSSESnapshot(w, page) {
				return
			}
			flusher.Flush()
		}
	}
}

func setSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Header().Set("X-Accel-Buffering", "no")
}

func writeSSESnapshot(w http.ResponseWriter, page resources.Page) bool {
	return writeSSEEvent(w, "snapshot", page)
}

func writeSSEEvent(w http.ResponseWriter, event string, value any) bool {
	payload, err := json.Marshal(value)
	if err != nil {
		return false
	}
	_, err = w.Write([]byte("event: " + event + "\ndata: " + string(payload) + "\n\n"))
	return err == nil
}

func (s *Server) resourceList(w http.ResponseWriter, r *http.Request, tenant config.TenantConfig) {
	resourceType, ok := s.requestedType(r)
	if !ok {
		writeError(w, http.StatusNotFound, "configured resource type not found")
		return
	}
	if !resourceType.Available {
		s.writeUnavailableType(w, &resourceType)
		return
	}
	query, err := resourceQuery(r, &resourceType)
	if err != nil {
		writeQueryError(w, err)
		return
	}
	page, err := s.resources.List(r.Context(), tenant.ID, resourceType.Key(), &query)
	if errors.Is(err, resources.ErrNotSynchronized) {
		s.writeObservationError(w, r, resourceType.Key())
		return
	}
	if err != nil {
		var validation *resources.QueryValidationError
		if errors.As(err, &validation) {
			writeQueryError(w, err)
			return
		}
		slog.Error("list resources", "error", err)
		writeError(w, http.StatusInternalServerError, "could not list resources")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"type": resourceType, "items": page.Items, "total": page.Total, "page": page.Page, "pageSize": page.PageSize, "pageCount": page.PageCount})
}

func resourceQuery(r *http.Request, resourceType *resources.Type) (resources.Query, error) {
	values := r.URL.Query()
	query := resources.Query{Name: values.Get("name"), Namespace: values.Get("namespace"), NameMode: resources.MatchMode(values.Get("nameMode")), NamespaceMode: resources.MatchMode(values.Get("namespaceMode")), JSONPath: values.Get("jsonPath"), JSONValue: values.Get("jsonValue"), JSONComparison: resources.JSONComparison(values.Get("jsonComparison")), Page: 1, Namespaced: resourceType.Scope == resources.ScopeNamespaced}
	if raw := values.Get("page"); raw != "" {
		page, err := strconv.Atoi(raw)
		if err != nil {
			return resources.Query{}, &resources.QueryValidationError{Fields: map[string]string{"page": "page must be a positive integer"}}
		}
		query.Page = page
	}
	return query, nil
}

func writeQueryError(w http.ResponseWriter, err error) {
	var validation *resources.QueryValidationError
	if errors.As(err, &validation) {
		writeJSON(w, http.StatusBadRequest, map[string]any{fieldError: validation.Error(), "fields": validation.Fields})
		return
	}
	writeError(w, http.StatusInternalServerError, "could not query resources")
}

func (s *Server) resourceDetail(w http.ResponseWriter, r *http.Request, tenant config.TenantConfig) {
	resourceType, ok := s.requestedType(r)
	if !ok {
		writeError(w, http.StatusNotFound, "configured resource type not found")
		return
	}
	if !resourceType.Available {
		s.writeUnavailableType(w, &resourceType)
		return
	}
	detail, err := s.resources.Get(r.Context(), tenant.ID, resourceType.Key(), r.PathValue("uid"))
	if errors.Is(err, resources.ErrNotSynchronized) {
		s.writeObservationError(w, r, resourceType.Key())
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "resource not found")
		return
	}
	if err != nil {
		slog.Error("get resource", "error", err)
		writeError(w, http.StatusInternalServerError, "could not load resource")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) writeUnavailableType(w http.ResponseWriter, resourceType *resources.Type) {
	status := resourceType.Observation
	if status.State == "" {
		status = resources.ObservationStatus{State: resources.ObservationUnavailable, Reason: resourceType.Reason}
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{fieldError: "resource observation is unavailable", "observation": status})
}

func (s *Server) writeObservationStream(w http.ResponseWriter, r *http.Request, resourceType *resources.Type) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		s.writeUnavailableType(w, resourceType)
		return
	}
	status, err := s.resources.ObservationStatus(r.Context(), resourceType.Key())
	if err != nil {
		status = resourceType.Observation
	}
	setSSEHeaders(w)
	if !writeSSEEvent(w, "observation", status) {
		return
	}
	flusher.Flush()
}

func (s *Server) writeObservationError(w http.ResponseWriter, r *http.Request, typeKey string) {
	status, err := s.resources.ObservationStatus(r.Context(), typeKey)
	if err != nil {
		status = resources.ObservationStatus{State: resources.ObservationUnavailable, Reason: "observation status could not be loaded"}
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "resource observation is not current", "observation": status})
}

func (s *Server) configuration(w http.ResponseWriter, r *http.Request) {
	types := s.catalog.Types()
	for i := range types {
		status, err := s.resources.ObservationStatus(r.Context(), types[i].Key())
		if err != nil {
			status = resources.ObservationStatus{State: resources.ObservationUnavailable, Reason: "observation status could not be loaded"}
		}
		types[i].Observation = status
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"apiVersion": "yak8sui.io/v1", "resourceTypes": types, "relationships": s.cfg.Observation.Relationships})
}

func (s *Server) requestedType(r *http.Request) (resources.Type, bool) {
	group := r.PathValue("group")
	if group == "_core" {
		group = ""
	}
	return s.catalog.Find(group, r.PathValue("version"), r.PathValue("resource"))
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	state, err := token()
	if err != nil {
		http.Error(w, "could not start sign-in", 500)
		return
	}
	verifier := oauth2.GenerateVerifier()
	nonce, err := token()
	if err != nil {
		http.Error(w, "could not start sign-in", 500)
		return
	}
	s.attemptsMu.Lock()
	for key, attempt := range s.attempts {
		if time.Now().After(attempt.expires) {
			delete(s.attempts, key)
		}
	}
	s.attempts[state] = loginAttempt{verifier: verifier, nonce: nonce, expires: time.Now().Add(10 * time.Minute)}
	s.attemptsMu.Unlock()
	s.setCookie(w, loginCookie, state, "/auth/callback", time.Time{}, 600)
	http.Redirect(w, r, s.oidc.AuthorizationURL(state, oauth2.S256ChallengeFromVerifier(verifier), nonce), http.StatusFound)
}

func (s *Server) callback(w http.ResponseWriter, r *http.Request) {
	if problem := r.URL.Query().Get("error"); problem != "" {
		s.authFailure(w, r, "Identity provider rejected sign-in: "+problem)
		return
	}
	cookie, err := r.Cookie(loginCookie)
	if err != nil || cookie.Value == "" || cookie.Value != r.URL.Query().Get("state") {
		s.authFailure(w, r, "Sign-in could not be verified. Please try again.")
		return
	}
	s.attemptsMu.Lock()
	attempt, ok := s.attempts[cookie.Value]
	delete(s.attempts, cookie.Value)
	s.attemptsMu.Unlock()
	s.clearCookie(w, loginCookie, "/auth/callback")
	if !ok || time.Now().After(attempt.expires) {
		s.authFailure(w, r, "Sign-in attempt expired. Please try again.")
		return
	}
	identity, err := s.oidc.Exchange(r.Context(), r.URL.Query().Get("code"), attempt.verifier, attempt.nonce)
	if err != nil {
		slog.Warn("OIDC callback failed", "error", err)
		s.authFailure(w, r, "Sign-in failed. Please try again.")
		return
	}
	session, err := s.sessions.Create(identity)
	if err != nil {
		http.Error(w, "could not create session", 500)
		return
	}
	s.setCookie(w, sessionCookie, session.ID, "/", session.ExpiresAt, 0)
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) authFailure(w http.ResponseWriter, r *http.Request, message string) {
	target := "/?authError=" + url.QueryEscape(message)
	http.Redirect(w, r, target, http.StatusFound)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if !s.sameOrigin(r) {
		writeError(w, 403, "cross-origin logout rejected")
		return
	}
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		s.sessions.Revoke(cookie.Value)
	}
	s.clearCookie(w, sessionCookie, "/")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	base, err := url.Parse(s.cfg.Server.PublicURL)
	return err == nil && origin == base.Scheme+"://"+base.Host
}

func (s *Server) clearCookie(w http.ResponseWriter, name, path string) {
	s.setCookie(w, name, "", path, time.Unix(1, 0), -1)
}

func (s *Server) setCookie(w http.ResponseWriter, name, value, path string, expires time.Time, maxAge int) {
	// #nosec G124 -- Secure is configuration-dependent to support explicitly configured local HTTP development; all other security attributes are always enforced.
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: path, HttpOnly: true, Secure: s.cfg.SecureCookie(), SameSite: http.SameSiteLaxMode, Expires: expires, MaxAge: maxAge})
}

type contextKey struct{}

func (s *Server) requireSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := s.currentSession(r)
		if !ok {
			s.clearCookie(w, sessionCookie, "/")
			writeError(w, 401, "authentication required")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), contextKey{}, session)))
	}
}

func (s *Server) currentSession(r *http.Request) (*auth.Session, bool) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return nil, false
	}
	return s.sessions.Get(cookie.Value)
}

func (s *Server) requireTenant(next func(http.ResponseWriter, *http.Request, config.TenantConfig)) http.HandlerFunc {
	return s.requireSession(func(w http.ResponseWriter, r *http.Request) {
		session, ok := sessionFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusInternalServerError, "session context unavailable")
			return
		}
		requested := r.PathValue("tenant")
		for _, tenant := range s.authorizedTenants(session.Identity.Groups) {
			if tenant.ID == requested {
				next(w, r, tenant)
				return
			}
		}
		writeError(w, 403, "tenant is not authorized")
	})
}

func (s *Server) sessionInfo(w http.ResponseWriter, r *http.Request) {
	session, ok := sessionFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusInternalServerError, "session context unavailable")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"user": map[string]string{fieldName: session.Identity.Name, "email": session.Identity.Email}, "expiresAt": session.ExpiresAt, "tenants": publicTenants(s.authorizedTenants(session.Identity.Groups))})
}

func (s *Server) tenantContext(w http.ResponseWriter, _ *http.Request, tenant config.TenantConfig) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"tenant": map[string]string{fieldID: tenant.ID, fieldName: tenant.Name}})
}

func sessionFromContext(ctx context.Context) (*auth.Session, bool) {
	session, ok := ctx.Value(contextKey{}).(*auth.Session)
	return session, ok && session != nil
}

func (s *Server) authorizedTenants(groups []string) []config.TenantConfig {
	allowed := map[string]bool{}
	for _, group := range groups {
		allowed[group] = true
	}
	result := []config.TenantConfig{}
	for _, tenant := range s.cfg.Tenants {
		for _, group := range tenant.Groups {
			if allowed[group] {
				result = append(result, tenant)
				break
			}
		}
	}
	return result
}

func publicTenants(tenants []config.TenantConfig) []map[string]string {
	result := make([]map[string]string, 0, len(tenants))
	for _, tenant := range tenants {
		result = append(result, map[string]string{fieldID: tenant.ID, fieldName: tenant.Name})
	}
	return result
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Warn("write JSON response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{fieldError: message})
}

func token() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func spaHandler(directory string) http.Handler {
	files := os.DirFS(directory)
	fileServer := http.FileServer(http.FS(files))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(filepath.Clean(r.URL.Path), "/")
		if path == "." {
			path = "index.html"
		}
		if _, err := fs.Stat(files, path); errors.Is(err, fs.ErrNotExist) {
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, r2)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
