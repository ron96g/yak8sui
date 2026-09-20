package resources

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"

	"sigs.k8s.io/yaml"

	_ "modernc.org/sqlite"
)

var ErrNotSynchronized = errors.New("resource observation is not synchronized")

type ObservationState string

const (
	ObservationCurrent         ObservationState = "current"
	ObservationStale           ObservationState = "stale"
	ObservationResynchronizing ObservationState = "resynchronizing"
	ObservationUnavailable     ObservationState = "unavailable"
)

type ObservationStatus struct {
	State  ObservationState `json:"state"`
	Reason string           `json:"reason,omitempty"`
}

func (s ObservationStatus) Synchronized() bool { return s.State == ObservationCurrent }

type ObservedResource struct {
	UID, Namespace, Name, ResourceVersion string
	Contents                              []byte
	TenantIDs                             []string
}

const PageSize = 100

type MatchMode string

const (
	MatchFuzzy MatchMode = "fuzzy"
	MatchRegex MatchMode = "regex"
)

type Query struct {
	Name, Namespace         string
	NameMode, NamespaceMode MatchMode
	JSONPath, JSONValue     string
	JSONComparison          JSONComparison
	Page                    int
	Namespaced              bool
	ClampPage               bool
}

type Page struct {
	Items     []Summary `json:"items"`
	Total     int       `json:"total"`
	Page      int       `json:"page"`
	PageSize  int       `json:"pageSize"`
	PageCount int       `json:"pageCount"`
}

type QueryValidationError struct {
	Fields map[string]string `json:"fields"`
}

func (e *QueryValidationError) Error() string { return "invalid resource query" }

type Summary struct {
	UID       string `json:"uid"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
	Presentation
}

type Detail struct {
	UID             string `json:"uid"`
	Name            string `json:"name"`
	Namespace       string `json:"namespace,omitempty"`
	ResourceVersion string `json:"resourceVersion,omitempty"`
	TriggeredAt     string `json:"triggeredAt,omitempty"`
	YAML            string `json:"yaml"`
	Presentation
}

type Projection struct {
	db *sql.DB

	liveMu       sync.Mutex
	nextLiveID   uint64
	subscribers  map[string]map[uint64]chan struct{}
	snapshotHook func()
}

type scopedQuery struct {
	statement string
	arguments []any
	fallback  *compiledJSONFilter
}

// Subscription carries type-level invalidations. A closed Updates channel means
// the consumer fell behind and must reconnect for a replacement snapshot.
type Subscription struct {
	Updates <-chan struct{}
	close   func()
}

func (s *Subscription) Close() { s.close() }

func Open(path string) (*Projection, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open SQLite projection: %w", err)
	}
	db.SetMaxOpenConns(1)
	p := &Projection{db: db, subscribers: map[string]map[uint64]chan struct{}{}}
	if err := p.migrate(context.Background()); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}
		return nil, err
	}
	// A cleanly persisted "current" marker is not proof after process restart;
	// every type must cross a new list boundary before contents are served.
	if _, err := p.db.Exec(`UPDATE observation_scopes SET synchronized=0,state=?,reason=?`, ObservationResynchronizing, "backend restarted; waiting for authoritative relist"); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, errors.Join(fmt.Errorf("invalidate persisted observation freshness: %w", err), closeErr)
		}
		return nil, fmt.Errorf("invalidate persisted observation freshness: %w", err)
	}
	return p, nil
}

func (p *Projection) Close() error { return p.db.Close() }

func (p *Projection) migrate(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS resources (
  type_key TEXT NOT NULL, uid TEXT NOT NULL, namespace TEXT NOT NULL,
  name TEXT NOT NULL, resource_version TEXT NOT NULL, contents BLOB NOT NULL,
  PRIMARY KEY (type_key, uid)
);
CREATE UNIQUE INDEX IF NOT EXISTS resources_name ON resources(type_key, namespace, name);
CREATE TABLE IF NOT EXISTS resource_tenants (
  type_key TEXT NOT NULL, uid TEXT NOT NULL, tenant_id TEXT NOT NULL,
  PRIMARY KEY (type_key, uid, tenant_id),
  FOREIGN KEY (type_key, uid) REFERENCES resources(type_key, uid) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS resource_tenant_scope ON resource_tenants(tenant_id, type_key, uid);
CREATE TABLE IF NOT EXISTS resource_labels (
  type_key TEXT NOT NULL, uid TEXT NOT NULL, label_key TEXT NOT NULL, label_value TEXT NOT NULL,
  PRIMARY KEY (type_key, uid, label_key),
  FOREIGN KEY (type_key, uid) REFERENCES resources(type_key, uid) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS resource_label_lookup ON resource_labels(type_key, label_key, label_value, uid);
CREATE TABLE IF NOT EXISTS projection_metadata (key TEXT PRIMARY KEY, value TEXT NOT NULL);
INSERT OR IGNORE INTO resource_labels(type_key,uid,label_key,label_value)
SELECT resources.type_key,resources.uid,labels.key,CAST(labels.value AS TEXT)
FROM resources,json_each(resources.contents,'$.metadata.labels') AS labels
WHERE labels.type='text'
  AND NOT EXISTS (SELECT 1 FROM projection_metadata WHERE key='resource_labels_backfilled');
INSERT OR IGNORE INTO projection_metadata(key,value) VALUES('resource_labels_backfilled','1');
CREATE TABLE IF NOT EXISTS resource_owner_references (
  child_type_key TEXT NOT NULL, child_uid TEXT NOT NULL, ordinal INTEGER NOT NULL,
  parent_api_version TEXT NOT NULL, parent_kind TEXT NOT NULL, parent_name TEXT NOT NULL, parent_uid TEXT NOT NULL,
  controller INTEGER NOT NULL,
  PRIMARY KEY (child_type_key, child_uid, ordinal),
  FOREIGN KEY (child_type_key, child_uid) REFERENCES resources(type_key, uid) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS resource_owner_reference_lookup
  ON resource_owner_references(child_type_key,parent_api_version,parent_kind,parent_uid,child_uid);
CREATE TABLE IF NOT EXISTS observation_scopes (
  type_key TEXT PRIMARY KEY, synchronized INTEGER NOT NULL,
  state TEXT NOT NULL DEFAULT 'resynchronizing', reason TEXT NOT NULL DEFAULT ''
);
PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;`
	if _, err := p.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("initialize SQLite projection: %w", err)
	}
	if err := p.backfillOwnerReferences(ctx); err != nil {
		return err
	}
	// These additive migrations preserve databases created before observation
	// freshness had explicit states. SQLite has no IF NOT EXISTS for columns.
	for _, statement := range []string{
		`ALTER TABLE observation_scopes ADD COLUMN state TEXT NOT NULL DEFAULT 'resynchronizing'`,
		`ALTER TABLE observation_scopes ADD COLUMN reason TEXT NOT NULL DEFAULT ''`,
	} {
		if _, err := p.db.ExecContext(ctx, statement); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return fmt.Errorf("migrate observation scope: %w", err)
		}
	}
	return nil
}

func (p *Projection) backfillOwnerReferences(ctx context.Context) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin owner reference backfill: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			slog.Warn("rollback owner reference backfill", "error", rollbackErr)
		}
	}()
	const statement = `INSERT OR IGNORE INTO resource_owner_references(child_type_key,child_uid,ordinal,parent_api_version,parent_kind,parent_name,parent_uid,controller)
SELECT resources.type_key,resources.uid,CAST(refs.key AS INTEGER),
       COALESCE(json_extract(refs.value,'$.apiVersion'),''),
       COALESCE(json_extract(refs.value,'$.kind'),''),
       COALESCE(json_extract(refs.value,'$.name'),''),
       COALESCE(json_extract(refs.value,'$.uid'),''),
       CASE WHEN json_extract(refs.value,'$.controller') THEN 1 ELSE 0 END
FROM resources,json_each(resources.contents,'$.metadata.ownerReferences') AS refs
WHERE refs.type='object'
  AND NOT EXISTS (SELECT 1 FROM projection_metadata WHERE key='resource_owner_references_backfilled')`
	if _, err = tx.ExecContext(ctx, statement); err != nil {
		return fmt.Errorf("backfill owner references: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO projection_metadata(key,value) VALUES('resource_owner_references_backfilled','1')`); err != nil {
		return fmt.Errorf("mark owner reference backfill complete: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit owner reference backfill: %w", err)
	}
	return nil
}

func (p *Projection) SetSynchronized(ctx context.Context, typeKey string, synchronized bool) error {
	state := ObservationResynchronizing
	if synchronized {
		state = ObservationCurrent
	}
	return p.SetObservationStatus(ctx, typeKey, ObservationStatus{State: state})
}

func (p *Projection) SetObservationStatus(ctx context.Context, typeKey string, status ObservationStatus) error {
	p.liveMu.Lock()
	defer p.liveMu.Unlock()
	_, err := p.db.ExecContext(ctx, `INSERT INTO observation_scopes(type_key,synchronized,state,reason) VALUES (?,?,?,?)
ON CONFLICT(type_key) DO UPDATE SET synchronized=excluded.synchronized,state=excluded.state,reason=excluded.reason`, typeKey, status.Synchronized(), status.State, status.Reason)
	if err == nil {
		p.publishLocked(typeKey)
	}
	return err
}

func (p *Projection) ObservationStatus(ctx context.Context, typeKey string) (ObservationStatus, error) {
	var status ObservationStatus
	err := p.db.QueryRowContext(ctx, `SELECT state,reason FROM observation_scopes WHERE type_key=?`, typeKey).Scan(&status.State, &status.Reason)
	if errors.Is(err, sql.ErrNoRows) {
		return ObservationStatus{State: ObservationResynchronizing, Reason: "waiting for initial synchronization"}, nil
	}
	return status, err
}

// ReplaceSnapshot atomically replaces one type's complete projection and marks
// it current. No reader or subscriber can observe a partially reconciled relist.
func (p *Projection) ReplaceSnapshot(ctx context.Context, typeKey string, resources []ObservedResource) error {
	p.liveMu.Lock()
	defer p.liveMu.Unlock()
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			slog.Warn("rollback snapshot transaction", "error", rollbackErr)
		}
	}()
	if _, err = tx.ExecContext(ctx, `DELETE FROM resources WHERE type_key=?`, typeKey); err != nil {
		return err
	}
	for _, resource := range resources {
		if _, err = tx.ExecContext(ctx, `INSERT INTO resources(type_key,uid,namespace,name,resource_version,contents) VALUES(?,?,?,?,?,?)`, typeKey, resource.UID, resource.Namespace, resource.Name, resource.ResourceVersion, resource.Contents); err != nil {
			return err
		}
		if labelErr := replaceResourceLabels(ctx, tx, typeKey, resource.UID, resource.Contents); labelErr != nil {
			return labelErr
		}
		if ownerErr := replaceResourceOwnerReferences(ctx, tx, typeKey, resource.UID, resource.Contents); ownerErr != nil {
			return ownerErr
		}
		for _, tenantID := range resource.TenantIDs {
			if _, err = tx.ExecContext(ctx, `INSERT INTO resource_tenants(type_key,uid,tenant_id) VALUES(?,?,?)`, typeKey, resource.UID, tenantID); err != nil {
				return err
			}
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO observation_scopes(type_key,synchronized,state,reason) VALUES (?,1,?, '')
ON CONFLICT(type_key) DO UPDATE SET synchronized=1,state=excluded.state,reason=''`, typeKey, ObservationCurrent); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	p.publishLocked(typeKey)
	return nil
}

func (p *Projection) Replace(ctx context.Context, typeKey, uid, namespace, name, resourceVersion string, contents []byte, tenantIDs []string) error {
	p.liveMu.Lock()
	defer p.liveMu.Unlock()
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			// Commit remains the authoritative return; rollback only releases an uncommitted transaction.
			return
		}
	}()
	// A same-name Kubernetes recreation replaces the old UID immediately.
	if _, err = tx.ExecContext(ctx, `DELETE FROM resources WHERE type_key=? AND namespace=? AND name=? AND uid<>?`, typeKey, namespace, name, uid); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO resources(type_key,uid,namespace,name,resource_version,contents)
VALUES(?,?,?,?,?,?) ON CONFLICT(type_key,uid) DO UPDATE SET namespace=excluded.namespace,name=excluded.name,resource_version=excluded.resource_version,contents=excluded.contents`, typeKey, uid, namespace, name, resourceVersion, contents); err != nil {
		return err
	}
	if labelErr := replaceResourceLabels(ctx, tx, typeKey, uid, contents); labelErr != nil {
		return labelErr
	}
	if ownerErr := replaceResourceOwnerReferences(ctx, tx, typeKey, uid, contents); ownerErr != nil {
		return ownerErr
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM resource_tenants WHERE type_key=? AND uid=?`, typeKey, uid); err != nil {
		return err
	}
	for _, tenantID := range tenantIDs {
		if _, err = tx.ExecContext(ctx, `INSERT INTO resource_tenants(type_key,uid,tenant_id) VALUES(?,?,?)`, typeKey, uid, tenantID); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	p.publishLocked(typeKey)
	return nil
}

func (p *Projection) Delete(ctx context.Context, typeKey, uid string) error {
	p.liveMu.Lock()
	defer p.liveMu.Unlock()
	result, err := p.db.ExecContext(ctx, `DELETE FROM resources WHERE type_key=? AND uid=?`, typeKey, uid)
	if err == nil {
		if affected, affectedErr := result.RowsAffected(); affectedErr == nil && affected > 0 {
			p.publishLocked(typeKey)
		}
	}
	return err
}

func (p *Projection) DeleteExcept(ctx context.Context, typeKey string, retainedUIDs []string) error {
	p.liveMu.Lock()
	defer p.liveMu.Unlock()
	if len(retainedUIDs) == 0 {
		result, err := p.db.ExecContext(ctx, `DELETE FROM resources WHERE type_key=?`, typeKey)
		if err == nil {
			if affected, affectedErr := result.RowsAffected(); affectedErr == nil && affected > 0 {
				p.publishLocked(typeKey)
			}
		}
		return err
	}
	query := `DELETE FROM resources WHERE type_key=? AND uid NOT IN (`
	arguments := make([]any, 0, len(retainedUIDs)+1)
	arguments = append(arguments, typeKey)
	for i, uid := range retainedUIDs {
		if i > 0 {
			query += ","
		}
		query += "?"
		arguments = append(arguments, uid)
	}
	result, err := p.db.ExecContext(ctx, query+")", arguments...)
	if err == nil {
		if affected, affectedErr := result.RowsAffected(); affectedErr == nil && affected > 0 {
			p.publishLocked(typeKey)
		}
	}
	return err
}

// SnapshotAndSubscribe registers the subscription in the same publication
// critical section as its initial snapshot. A commit is therefore either
// represented by the snapshot or produces a subsequent invalidation.
func (p *Projection) SnapshotAndSubscribe(ctx context.Context, tenantID, typeKey string, query *Query) (Page, *Subscription, error) {
	p.liveMu.Lock()
	defer p.liveMu.Unlock()
	page, err := p.List(ctx, tenantID, typeKey, query)
	if err != nil {
		return Page{}, nil, err
	}
	if p.snapshotHook != nil {
		p.snapshotHook()
	}
	p.nextLiveID++
	id := p.nextLiveID
	updates := make(chan struct{}, 1)
	if p.subscribers[typeKey] == nil {
		p.subscribers[typeKey] = map[uint64]chan struct{}{}
	}
	p.subscribers[typeKey][id] = updates
	return page, &Subscription{Updates: updates, close: func() { p.unsubscribe(typeKey, id, updates) }}, nil
}

// DetailSnapshotAndSubscribe creates a UID-scoped replacement subscription in
// the same commit critical section as its initial authorized read.
func (p *Projection) DetailSnapshotAndSubscribe(ctx context.Context, tenantID, typeKey, uid string) (Detail, *Subscription, error) {
	p.liveMu.Lock()
	defer p.liveMu.Unlock()
	detail, err := p.Get(ctx, tenantID, typeKey, uid)
	if err != nil {
		return Detail{}, nil, err
	}
	if p.snapshotHook != nil {
		p.snapshotHook()
	}
	p.nextLiveID++
	id := p.nextLiveID
	updates := make(chan struct{}, 1)
	if p.subscribers[typeKey] == nil {
		p.subscribers[typeKey] = map[uint64]chan struct{}{}
	}
	p.subscribers[typeKey][id] = updates
	return detail, &Subscription{Updates: updates, close: func() { p.unsubscribe(typeKey, id, updates) }}, nil
}

func (p *Projection) publishLocked(typeKey string) {
	p.publishSubscribersLocked(typeKey)
}

func (p *Projection) publishSubscribersLocked(typeKey string) {
	for id, updates := range p.subscribers[typeKey] {
		select {
		case updates <- struct{}{}:
		default:
			// A slow consumer must replace its view from a fresh snapshot rather
			// than miss an invalidation or apply an inconsistent partial history.
			for _, subscribers := range p.subscribers {
				if subscribers[id] == updates {
					delete(subscribers, id)
				}
			}
			close(updates)
		}
	}
}

func (p *Projection) unsubscribe(typeKey string, id uint64, expected chan struct{}) {
	p.liveMu.Lock()
	defer p.liveMu.Unlock()
	updates, ok := p.subscribers[typeKey][id]
	if !ok || updates != expected {
		return
	}
	delete(p.subscribers[typeKey], id)
	close(updates)
}

func (p *Projection) List(ctx context.Context, tenantID, typeKey string, query *Query) (Page, error) {
	if err := p.requireSynchronized(ctx, typeKey); err != nil {
		return Page{}, err
	}
	matchName, matchNamespace, jsonFilter, err := compileQuery(query)
	if err != nil {
		return Page{}, err
	}
	scoped := sqliteScopedQuery(tenantID, typeKey, jsonFilter)
	rows, err := p.db.QueryContext(ctx, scoped.statement, scoped.arguments...)
	if err != nil {
		return Page{}, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			slog.Warn("close resource rows", "error", closeErr)
		}
	}()
	matches := []Summary{}
	for rows.Next() {
		var item Summary
		var contents []byte
		if err := rows.Scan(&item.UID, &item.Name, &item.Namespace, &item.CreatedAt, &contents); err != nil {
			return Page{}, err
		}
		if !matchName(item.Name) || (query.Namespaced && !matchNamespace(item.Namespace)) || (scoped.fallback != nil && !scoped.fallback.matches(contents)) {
			continue
		}
		item.Presentation = present(contents)
		matches = append(matches, item)
	}
	if err := rows.Err(); err != nil {
		return Page{}, err
	}
	return paginateMatches(matches, query)
}

func paginateMatches(matches []Summary, query *Query) (Page, error) {
	total := len(matches)
	pageCount := (total + PageSize - 1) / PageSize
	page := query.Page
	if page == 0 {
		page = 1
	}
	if query.ClampPage && page > max(pageCount, 1) {
		page = max(pageCount, 1)
	}
	if page < 1 || (pageCount > 0 && page > pageCount) || (pageCount == 0 && page != 1) {
		return Page{}, &QueryValidationError{Fields: map[string]string{"page": "page is outside the filtered result set"}}
	}
	start := min((page-1)*PageSize, total)
	end := min(start+PageSize, total)
	return Page{Items: slices.Clone(matches[start:end]), Total: total, Page: page, PageSize: PageSize, PageCount: pageCount}, nil
}

func sqliteScopedQuery(tenantID, typeKey string, jsonFilter *compiledJSONFilter) scopedQuery {
	const prefix = `SELECT r.uid,r.name,r.namespace,COALESCE(json_extract(r.contents,'$.metadata.creationTimestamp'),''),r.contents FROM resource_tenants AS m INDEXED BY resource_tenant_scope
CROSS JOIN resources AS r INDEXED BY sqlite_autoindex_resources_1
WHERE m.tenant_id=? AND m.type_key=? AND r.type_key=m.type_key AND r.uid=m.uid`
	result := scopedQuery{statement: prefix, arguments: []any{tenantID, typeKey}}
	if jsonFilter == nil {
		result.statement += " ORDER BY r.namespace,r.name"
		return result
	}
	if jsonFilter.fallback {
		result.fallback = jsonFilter
	} else {
		// jsonFilter.sql is emitted only by buildSQLiteJSONPredicate from fixed
		// fragments; every user-provided path segment and value is bound below.
		result.statement += " AND " + jsonFilter.sql // #nosec G202
		result.arguments = append(result.arguments, jsonFilter.args...)
	}
	result.statement += " ORDER BY r.namespace,r.name"
	return result
}

func compileQuery(query *Query) (func(string) bool, func(string) bool, *compiledJSONFilter, error) {
	errorsByField := map[string]string{}
	name := compileMatcher(query.Name, query.NameMode, "name", errorsByField)
	namespace := func(string) bool { return true }
	if query.Namespaced {
		namespace = compileMatcher(query.Namespace, query.NamespaceMode, "namespace", errorsByField)
	}
	jsonFilter := compileQueryJSONFilter(query, errorsByField)
	if len(errorsByField) > 0 {
		return nil, nil, nil, &QueryValidationError{Fields: errorsByField}
	}
	return name, namespace, jsonFilter, nil
}

func compileQueryJSONFilter(query *Query, errorsByField map[string]string) *compiledJSONFilter {
	if query.JSONPath == "" {
		if query.JSONComparison == JSONEquals || query.JSONValue != "" {
			errorsByField["jsonPath"] = "JSONPath is required for a comparison"
		}
		return nil
	}
	filter, err := compileJSONFilter(query.JSONPath, query.JSONValue, query.JSONComparison)
	if err == nil {
		return filter
	}
	field := "jsonPath"
	if query.JSONComparison == JSONEquals && strings.Contains(err.Error(), "value") {
		field = "jsonValue"
	}
	if strings.Contains(err.Error(), "comparison") {
		field = "jsonComparison"
	}
	errorsByField[field] = err.Error()
	return nil
}

func compileMatcher(pattern string, mode MatchMode, field string, errorsByField map[string]string) func(string) bool {
	if mode == "" {
		mode = MatchFuzzy
	}
	if mode != MatchFuzzy && mode != MatchRegex {
		errorsByField[field] = "mode must be fuzzy or regex"
		return func(string) bool { return false }
	}
	if pattern == "" {
		return func(string) bool { return true }
	}
	switch mode {
	case MatchFuzzy:
		needle := []rune(strings.ToLower(pattern))
		return func(value string) bool {
			position := 0
			for _, candidate := range strings.ToLower(value) {
				if position < len(needle) && candidate == needle[position] {
					position++
				}
			}
			return position == len(needle)
		}
	case MatchRegex:
		expression, err := regexp.Compile(pattern)
		if err != nil {
			errorsByField[field] = "invalid Go regular expression: " + err.Error()
			return func(string) bool { return false }
		}
		return expression.MatchString
	}
	return func(string) bool { return false }
}

func (p *Projection) Get(ctx context.Context, tenantID, typeKey, uid string) (Detail, error) {
	if err := p.requireSynchronized(ctx, typeKey); err != nil {
		return Detail{}, err
	}
	var detail Detail
	var contents []byte
	err := p.db.QueryRowContext(ctx, `SELECT r.uid,r.name,r.namespace,r.contents FROM resource_tenants m
JOIN resources r ON r.type_key=m.type_key AND r.uid=m.uid
WHERE m.tenant_id=? AND m.type_key=? AND m.uid=?`, tenantID, typeKey, uid).Scan(&detail.UID, &detail.Name, &detail.Namespace, &contents)
	if err != nil {
		return Detail{}, err
	}
	if !json.Valid(contents) {
		return Detail{}, errors.New("stored resource contents are invalid JSON")
	}
	var metadata struct {
		Metadata struct {
			ResourceVersion string            `json:"resourceVersion"`
			Annotations     map[string]string `json:"annotations"`
		} `json:"metadata"`
	}
	if unmarshalErr := json.Unmarshal(contents, &metadata); unmarshalErr != nil {
		return Detail{}, fmt.Errorf("read resource metadata: %w", unmarshalErr)
	}
	detail.ResourceVersion = metadata.Metadata.ResourceVersion
	detail.TriggeredAt = metadata.Metadata.Annotations[TriggeredAtAnnotation]
	yamlContents, err := yaml.JSONToYAML(contents)
	if err != nil {
		return Detail{}, fmt.Errorf("render resource YAML: %w", err)
	}
	detail.YAML = string(yamlContents)
	detail.Presentation = present(contents)
	return detail, nil
}

func (p *Projection) authorizedObject(ctx context.Context, tenantID, typeKey, uid string) (projectedObject, error) {
	if err := p.requireSynchronized(ctx, typeKey); err != nil {
		return projectedObject{}, err
	}
	var object projectedObject
	err := p.db.QueryRowContext(ctx, `SELECT r.uid,r.namespace,r.name,r.contents FROM resource_tenants m JOIN resources r ON r.type_key=m.type_key AND r.uid=m.uid WHERE m.tenant_id=? AND m.type_key=? AND m.uid=?`, tenantID, typeKey, uid).Scan(&object.UID, &object.Namespace, &object.Name, &object.Contents)
	return object, err
}

func (p *Projection) authorizedObjectByName(ctx context.Context, tenantID, typeKey, namespace, name string) (projectedObject, error) {
	if err := p.requireSynchronized(ctx, typeKey); err != nil {
		return projectedObject{}, err
	}
	var object projectedObject
	err := p.db.QueryRowContext(ctx, `SELECT r.uid,r.namespace,r.name,r.contents
FROM resource_tenants AS m INDEXED BY resource_tenant_scope
CROSS JOIN resources AS r INDEXED BY resources_name
WHERE m.tenant_id=? AND m.type_key=? AND r.type_key=m.type_key AND r.uid=m.uid AND r.namespace=? AND r.name=?`, tenantID, typeKey, namespace, name).Scan(&object.UID, &object.Namespace, &object.Name, &object.Contents)
	return object, err
}

type sqlExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func replaceResourceLabels(ctx context.Context, db sqlExecutor, typeKey, uid string, contents []byte) error {
	var object struct {
		Metadata struct {
			Labels map[string]string `json:"labels"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(contents, &object); err != nil {
		return fmt.Errorf("decode resource labels: %w", err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM resource_labels WHERE type_key=? AND uid=?`, typeKey, uid); err != nil {
		return err
	}
	for key, value := range object.Metadata.Labels {
		if _, err := db.ExecContext(ctx, `INSERT INTO resource_labels(type_key,uid,label_key,label_value) VALUES(?,?,?,?)`, typeKey, uid, key, value); err != nil {
			return err
		}
	}
	return nil
}

func replaceResourceOwnerReferences(ctx context.Context, db sqlExecutor, typeKey, uid string, contents []byte) error {
	refs, err := decodeOwnerReferences(contents)
	if err != nil {
		return err
	}
	if _, err = db.ExecContext(ctx, `DELETE FROM resource_owner_references WHERE child_type_key=? AND child_uid=?`, typeKey, uid); err != nil {
		return err
	}
	for ordinal, ref := range refs {
		controller := ref.Controller != nil && *ref.Controller
		if _, err = db.ExecContext(ctx, `INSERT INTO resource_owner_references(child_type_key,child_uid,ordinal,parent_api_version,parent_kind,parent_name,parent_uid,controller) VALUES(?,?,?,?,?,?,?,?)`, typeKey, uid, ordinal, ref.APIVersion, ref.Kind, ref.Name, ref.UID, controller); err != nil {
			return err
		}
	}
	return nil
}

func (p *Projection) authorizedOwnerReferenceChildren(ctx context.Context, tenantID, childTypeKey, parentAPIVersion, parentKind, parentUID string) ([]projectedObject, map[string]bool, error) {
	if err := p.requireSynchronized(ctx, childTypeKey); err != nil {
		return nil, nil, err
	}
	rows, err := p.db.QueryContext(ctx, `SELECT r.uid,r.namespace,r.name,r.contents,o.controller
FROM resource_owner_references AS o INDEXED BY resource_owner_reference_lookup
CROSS JOIN resource_tenants AS m INDEXED BY resource_tenant_scope
CROSS JOIN resources AS r INDEXED BY sqlite_autoindex_resources_1
WHERE o.child_type_key=? AND o.parent_api_version=? AND o.parent_kind=? AND o.parent_uid=?
AND m.tenant_id=? AND m.type_key=o.child_type_key AND m.uid=o.child_uid
AND r.type_key=o.child_type_key AND r.uid=o.child_uid`, childTypeKey, parentAPIVersion, parentKind, parentUID, tenantID)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			slog.Warn("close owner reference rows", "error", closeErr)
		}
	}()
	var result []projectedObject
	controllers := map[string]bool{}
	for rows.Next() {
		var item projectedObject
		var controller bool
		if err := rows.Scan(&item.UID, &item.Namespace, &item.Name, &item.Contents, &controller); err != nil {
			return nil, nil, err
		}
		result = append(result, item)
		controllers[item.UID] = controllers[item.UID] || controller
	}
	return result, controllers, rows.Err()
}

func (p *Projection) authorizedFieldReferenceChildren(ctx context.Context, tenantID, childTypeKey, namespace, path string, ref *fieldReference) ([]projectedObject, bool, error) {
	if err := p.requireSynchronized(ctx, childTypeKey); err != nil {
		return nil, false, err
	}
	base := ` FROM resource_tenants AS m INDEXED BY resource_tenant_scope CROSS JOIN resources AS r INDEXED BY sqlite_autoindex_resources_1
WHERE m.tenant_id=? AND m.type_key=? AND r.type_key=m.type_key AND r.uid=m.uid`
	baseArgs := []any{tenantID, childTypeKey}
	if namespace != "" {
		base += ` AND r.namespace=?`
		baseArgs = append(baseArgs, namespace)
	}
	present := `json_type(r.contents,?) IS NOT NULL`
	shape := `json_type(r.contents,?)='object'
AND json_type(r.contents,?||'.apiVersion')='text' AND json_extract(r.contents,?||'.apiVersion')=?
AND json_type(r.contents,?||'.kind')='text' AND json_extract(r.contents,?||'.kind')=?
AND json_type(r.contents,?||'.name')='text' AND json_extract(r.contents,?||'.name')<>''
AND json_type(r.contents,?||'.uid')='text' AND json_extract(r.contents,?||'.uid')<>''`
	shapeArgs := []any{path, path, path, ref.APIVersion, path, path, ref.Kind, path, path, path, path}
	if ref.Namespace != "" {
		shape += ` AND json_type(r.contents,?||'.namespace')='text' AND json_extract(r.contents,?||'.namespace')<>''`
		shapeArgs = append(shapeArgs, path, path)
	} else {
		shape += ` AND COALESCE(json_extract(r.contents,?||'.namespace'),'')=''`
		shapeArgs = append(shapeArgs, path)
	}
	exact := shape + ` AND json_extract(r.contents,?||'.name')=? AND json_extract(r.contents,?||'.uid')=?`
	exactArgs := append(append([]any{}, shapeArgs...), path, ref.Name, path, ref.UID)
	if ref.Namespace != "" {
		exact += ` AND json_extract(r.contents,?||'.namespace')=?`
		exactArgs = append(exactArgs, path, ref.Namespace)
	}
	queryArgs := append(append([]any{}, baseArgs...), exactArgs...)
	rows, err := p.db.QueryContext(ctx, `SELECT r.uid,r.namespace,r.name,r.contents`+base+` AND `+exact, queryArgs...)
	if err != nil {
		return nil, false, err
	}
	var result []projectedObject
	for rows.Next() {
		var item projectedObject
		if scanErr := rows.Scan(&item.UID, &item.Namespace, &item.Name, &item.Contents); scanErr != nil {
			if closeErr := rows.Close(); closeErr != nil {
				slog.Warn("close field reference rows after scan failure", "error", closeErr)
			}
			return nil, false, scanErr
		}
		result = append(result, item)
	}
	if closeErr := rows.Close(); closeErr != nil {
		return nil, false, closeErr
	}
	identityMismatch := ` OR (` + shape + ` AND ((json_extract(r.contents,?||'.uid')=? AND json_extract(r.contents,?||'.name')<>?) OR (json_extract(r.contents,?||'.name')=? AND json_extract(r.contents,?||'.uid')<>?)))`
	identityArgs := append(append([]any{}, shapeArgs...), path, ref.UID, path, ref.Name, path, ref.Name, path, ref.UID)
	if ref.Namespace != "" {
		identityMismatch = ` OR (` + shape + ` AND ((json_extract(r.contents,?||'.uid')=? AND (json_extract(r.contents,?||'.name')<>? OR json_extract(r.contents,?||'.namespace')<>?)) OR (json_extract(r.contents,?||'.name')=? AND (json_extract(r.contents,?||'.uid')<>? OR json_extract(r.contents,?||'.namespace')<>?))))`
		identityArgs = append(append([]any{}, shapeArgs...), path, ref.UID, path, ref.Name, path, ref.Namespace, path, ref.Name, path, ref.UID, path, ref.Namespace)
	}
	malformedArgs := append(append(append([]any{}, baseArgs...), path), shapeArgs...)
	malformedArgs = append(malformedArgs, identityArgs...)
	var malformed bool
	err = p.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1`+base+` AND `+present+` AND (NOT COALESCE((`+shape+`),0)`+identityMismatch+`))`, malformedArgs...).Scan(&malformed)
	return result, malformed, err
}

type labelMatch struct {
	key, value string
}

func labelLookupQuery(tenantID, typeKey, namespace string, labels []labelMatch, limit int) (string, []any) {
	sort.Slice(labels, func(i, j int) bool {
		if labels[i].key != labels[j].key {
			return labels[i].key < labels[j].key
		}
		return labels[i].value < labels[j].value
	})
	first := labels[0]
	statement := `SELECT r.uid,r.namespace,r.name,r.contents FROM resource_labels AS matched INDEXED BY resource_label_lookup
CROSS JOIN resource_tenants AS m INDEXED BY resource_tenant_scope
CROSS JOIN resources AS r INDEXED BY sqlite_autoindex_resources_1
WHERE matched.type_key=? AND matched.label_key=? AND matched.label_value=?
AND m.tenant_id=? AND m.type_key=matched.type_key AND m.uid=matched.uid
AND r.type_key=matched.type_key AND r.uid=matched.uid`
	arguments := []any{typeKey, first.key, first.value, tenantID}
	if namespace != "" {
		statement += ` AND r.namespace=?`
		arguments = append(arguments, namespace)
	}
	for _, label := range labels[1:] {
		statement += ` AND EXISTS (SELECT 1 FROM resource_labels AS l WHERE l.type_key=r.type_key AND l.uid=r.uid AND l.label_key=? AND l.label_value=?)`
		arguments = append(arguments, label.key, label.value)
	}
	if limit > 0 {
		statement += ` LIMIT ?`
		arguments = append(arguments, limit)
	}
	return statement, arguments
}

func (p *Projection) authorizedObjectsByLabels(ctx context.Context, tenantID, typeKey, namespace string, labels []labelMatch, limit int) ([]projectedObject, error) {
	if err := p.requireSynchronized(ctx, typeKey); err != nil {
		return nil, err
	}
	statement, arguments := labelLookupQuery(tenantID, typeKey, namespace, labels, limit)
	rows, err := p.db.QueryContext(ctx, statement, arguments...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			slog.Warn("close label relationship rows", "error", closeErr)
		}
	}()
	var result []projectedObject
	for rows.Next() {
		var item projectedObject
		if err := rows.Scan(&item.UID, &item.Namespace, &item.Name, &item.Contents); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (p *Projection) RelationshipSnapshotAndSubscribe(typeKeys []string, resolve func() (Neighborhood, error)) (Neighborhood, *Subscription, error) {
	p.liveMu.Lock()
	defer p.liveMu.Unlock()
	result, err := resolve()
	if err != nil {
		return Neighborhood{}, nil, err
	}
	p.nextLiveID++
	id := p.nextLiveID
	updates := make(chan struct{}, 1)
	for _, key := range typeKeys {
		if p.subscribers[key] == nil {
			p.subscribers[key] = map[uint64]chan struct{}{}
		}
		p.subscribers[key][id] = updates
	}
	return result, &Subscription{Updates: updates, close: func() {
		p.liveMu.Lock()
		defer p.liveMu.Unlock()
		found := false
		for _, key := range typeKeys {
			if p.subscribers[key][id] == updates {
				delete(p.subscribers[key], id)
				found = true
			}
		}
		if found {
			close(updates)
		}
	}}, nil
}

func (p *Projection) requireSynchronized(ctx context.Context, typeKey string) error {
	status, err := p.ObservationStatus(ctx, typeKey)
	if err == nil && !status.Synchronized() {
		return ErrNotSynchronized
	}
	return err
}
