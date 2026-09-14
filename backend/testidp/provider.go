// Package testidp provides a deliberately small OIDC provider for automated
// tests and controlled local demos only. It is not suitable for production.
package testidp

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"golang.org/x/oauth2"
)

type User struct {
	Subject, Name, Email string
	Groups               []string
}
type codeGrant struct {
	clientID, redirectURI, nonce, challenge string
	user                                    User
}

type Provider struct {
	Issuer      string
	key         *rsa.PrivateKey
	mu          sync.Mutex
	grants      map[string]codeGrant
	users       map[string]User
	defaultUser string
}

func New(issuer string, users map[string]User, defaultUser string) (*Provider, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	return &Provider{Issuer: issuer, key: key, grants: map[string]codeGrant{}, users: users, defaultUser: defaultUser}, nil
}
func (p *Provider) SetIssuer(issuer string) { p.Issuer = issuer }

func (p *Provider) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", p.discovery)
	mux.HandleFunc("GET /authorize", p.authorize)
	mux.HandleFunc("POST /token", p.exchange)
	mux.HandleFunc("GET /keys", p.keys)
	return mux
}

func (p *Provider) discovery(w http.ResponseWriter, _ *http.Request) {
	jsonResponse(w, map[string]any{"issuer": p.Issuer, "authorization_endpoint": p.Issuer + "/authorize", "token_endpoint": p.Issuer + "/token", "jwks_uri": p.Issuer + "/keys", "response_types_supported": []string{"code"}, "subject_types_supported": []string{"public"}, "id_token_signing_alg_values_supported": []string{"RS256"}, "code_challenge_methods_supported": []string{"S256"}})
}

func (p *Provider) keys(w http.ResponseWriter, _ *http.Request) {
	jsonResponse(w, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &p.key.PublicKey, KeyID: "test", Algorithm: "RS256", Use: "sig"}}})
}

func (p *Provider) authorize(w http.ResponseWriter, r *http.Request) {
	userName := p.defaultUser
	if cookie, err := r.Cookie("testidp_user"); err == nil {
		userName = cookie.Value
	}
	user, ok := p.users[userName]
	if !ok {
		http.Error(w, "unknown test user", 400)
		return
	}
	code, err := random()
	if err != nil {
		http.Error(w, "could not create authorization code", http.StatusInternalServerError)
		return
	}
	grant := codeGrant{clientID: r.URL.Query().Get("client_id"), redirectURI: r.URL.Query().Get("redirect_uri"), nonce: r.URL.Query().Get("nonce"), challenge: r.URL.Query().Get("code_challenge"), user: user}
	p.mu.Lock()
	p.grants[code] = grant
	p.mu.Unlock()
	target, err := url.Parse(grant.redirectURI)
	if err != nil {
		http.Error(w, "invalid redirect_uri", 400)
		return
	}
	query := target.Query()
	query.Set("code", code)
	query.Set("state", r.URL.Query().Get("state"))
	target.RawQuery = query.Encode()
	http.Redirect(w, r, target.String(), http.StatusFound)
}

func (p *Provider) exchange(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", 400)
		return
	}
	p.mu.Lock()
	code := r.Form.Get("code")
	grant, ok := p.grants[code]
	p.mu.Unlock()
	clientID := r.Form.Get("client_id")
	clientSecret := r.Form.Get("client_secret")
	if basicID, basicSecret, hasBasic := r.BasicAuth(); hasBasic {
		clientID = basicID
		clientSecret = basicSecret
	}
	if !ok || clientID != grant.clientID || clientSecret == "" {
		http.Error(w, "invalid grant", 400)
		return
	}
	if grant.challenge == "" || grant.challenge != oauth2.S256ChallengeFromVerifier(r.Form.Get("code_verifier")) {
		http.Error(w, "invalid PKCE verifier", 400)
		return
	}
	p.mu.Lock()
	delete(p.grants, code)
	p.mu.Unlock()
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: p.key, KeyID: "test"}}, nil)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	now := time.Now()
	claims := map[string]any{"iss": p.Issuer, "sub": grant.user.Subject, "aud": grant.clientID, "iat": now.Unix(), "exp": now.Add(time.Hour).Unix(), "nonce": grant.nonce, "name": grant.user.Name, "email": grant.user.Email, "groups": grant.user.Groups}
	raw, err := jwt.Signed(signer).Claims(claims).Serialize()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	jsonResponse(w, map[string]any{"access_token": "controlled-access-token", "token_type": "Bearer", "expires_in": 3600, "id_token": raw})
}

func random() (string, error) {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate authorization code: %w", err)
	}
	return fmt.Sprintf("%x", b), nil
}

func jsonResponse(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Warn("write test identity provider response", "error", err)
	}
}
