package resources

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTenantTypeScopeUsesProjectionIndexes(t *testing.T) {
	store, err := Open(t.TempDir() + "/projection.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	rows, err := store.db.QueryContext(t.Context(), `EXPLAIN QUERY PLAN SELECT r.uid FROM resource_tenants AS m INDEXED BY resource_tenant_scope
CROSS JOIN resources AS r INDEXED BY sqlite_autoindex_resources_1
WHERE m.tenant_id=? AND m.type_key=? AND r.type_key=m.type_key AND r.uid=m.uid ORDER BY r.namespace,r.name`, "platform", "widgets.example.test/v1/widgets")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	plan := ""
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		plan += detail + "\n"
	}
	if !strings.Contains(plan, "resource_tenant_scope") || !strings.Contains(plan, "sqlite_autoindex_resources_1") {
		t.Fatalf("query plan did not use tenant/type and resource identity indexes:\n%s", plan)
	}
}

func TestNativeJSONPredicateKeepsIndexedTenantTypeScope(t *testing.T) {
	store, err := Open(t.TempDir() + "/projection.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	filter, err := compileJSONFilter("$.status.conditions[*].type", `"Ready"`, JSONEquals)
	if err != nil {
		t.Fatal(err)
	}
	if filter.fallback || filter.sql == "" {
		t.Fatal("wildcard scalar equality did not compile to native SQLite traversal")
	}
	statement := `EXPLAIN QUERY PLAN SELECT r.uid FROM resource_tenants AS m INDEXED BY resource_tenant_scope
CROSS JOIN resources AS r INDEXED BY sqlite_autoindex_resources_1
WHERE m.tenant_id=? AND m.type_key=? AND r.type_key=m.type_key AND r.uid=m.uid AND ` + filter.sql
	arguments := make([]any, 0, 2+len(filter.args))
	arguments = append(arguments, "platform", "widgets.example.test/v1/widgets")
	arguments = append(arguments, filter.args...)
	rows, err := store.db.QueryContext(t.Context(), statement, arguments...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	plan := ""
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		plan += detail + "\n"
	}
	if !strings.Contains(plan, "resource_tenant_scope") {
		t.Fatalf("query plan did not combine indexed scope and native JSON traversal:\n%s", plan)
	}
	if !strings.Contains(filter.sql, "json_each") {
		t.Fatalf("compiled predicate did not use native JSON traversal: %s", filter.sql)
	}
}

func TestLabelLookupStartsFromIndexedLabelValueAndKeepsTenantScope(t *testing.T) {
	store, err := Open(t.TempDir() + "/projection.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	statement, arguments := labelLookupQuery("platform", "policies.example.test/v1/policies", "", []labelMatch{{key: "owner", value: "api"}, {key: "scope", value: "production"}}, 2)
	rows, err := store.db.QueryContext(t.Context(), "EXPLAIN QUERY PLAN "+statement, arguments...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	plan := ""
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		plan += detail + "\n"
	}
	for _, index := range []string{"resource_label_lookup", "resource_tenant_scope", "sqlite_autoindex_resources_1", "sqlite_autoindex_resource_labels_1"} {
		if !strings.Contains(plan, index) {
			t.Fatalf("label query plan did not use %s:\n%s", index, plan)
		}
	}
	if strings.Contains(plan, "TEMP B-TREE") {
		t.Fatalf("label query plan performs a temporary sort:\n%s", plan)
	}
}

func TestProjectionMaintainsQueryableResourceLabels(t *testing.T) {
	store, err := Open(t.TempDir() + "/projection.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	const typeKey = "policies.example.test/v1/policies"
	if syncErr := store.SetSynchronized(t.Context(), typeKey, true); syncErr != nil {
		t.Fatal(syncErr)
	}
	contents, _ := json.Marshal(map[string]any{"metadata": map[string]any{"uid": "parent", "name": "parent", "labels": map[string]any{"owner": "api", "scope": "production"}}})
	if replaceErr := store.Replace(t.Context(), typeKey, "parent", "", "parent", "1", contents, []string{"platform"}); replaceErr != nil {
		t.Fatal(replaceErr)
	}
	objects, err := store.authorizedObjectsByLabels(t.Context(), "platform", typeKey, "", []labelMatch{{key: "owner", value: "api"}, {key: "scope", value: "production"}}, 2)
	if err != nil || len(objects) != 1 || objects[0].UID != "parent" {
		t.Fatalf("matching labels = %#v, %v", objects, err)
	}
	contents, _ = json.Marshal(map[string]any{"metadata": map[string]any{"uid": "parent", "name": "parent", "labels": map[string]any{"owner": "other", "scope": "production"}}})
	if replaceErr := store.Replace(t.Context(), typeKey, "parent", "", "parent", "2", contents, []string{"platform"}); replaceErr != nil {
		t.Fatal(replaceErr)
	}
	objects, err = store.authorizedObjectsByLabels(t.Context(), "platform", typeKey, "", []labelMatch{{key: "owner", value: "api"}, {key: "scope", value: "production"}}, 2)
	if err != nil || len(objects) != 0 {
		t.Fatalf("stale label remained queryable: %#v, %v", objects, err)
	}
}

func TestOpenBackfillsLabelsFromExistingProjectionOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "projection.db")
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = legacy.Exec(`CREATE TABLE resources (type_key TEXT NOT NULL, uid TEXT NOT NULL, namespace TEXT NOT NULL, name TEXT NOT NULL, resource_version TEXT NOT NULL, contents BLOB NOT NULL, PRIMARY KEY(type_key,uid));
CREATE TABLE resource_tenants (type_key TEXT NOT NULL, uid TEXT NOT NULL, tenant_id TEXT NOT NULL, PRIMARY KEY(type_key,uid,tenant_id));
CREATE TABLE observation_scopes (type_key TEXT PRIMARY KEY, synchronized INTEGER NOT NULL, state TEXT NOT NULL DEFAULT 'resynchronizing', reason TEXT NOT NULL DEFAULT '');
INSERT INTO resources VALUES('policies.example.test/v1/policies','parent','','parent','1','{"metadata":{"labels":{"owner":"api"}}}');
INSERT INTO resource_tenants VALUES('policies.example.test/v1/policies','parent','platform');`)
	if err != nil {
		t.Fatal(err)
	}
	if closeErr := legacy.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	const typeKey = "policies.example.test/v1/policies"
	if syncErr := store.SetSynchronized(t.Context(), typeKey, true); syncErr != nil {
		t.Fatal(syncErr)
	}
	objects, err := store.authorizedObjectsByLabels(t.Context(), "platform", typeKey, "", []labelMatch{{key: "owner", value: "api"}}, 2)
	if err != nil || len(objects) != 1 {
		t.Fatalf("backfilled lookup = %#v, %v", objects, err)
	}
	if closeErr := store.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	store, err = Open(path)
	if err != nil {
		t.Fatalf("idempotent reopen: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
}

func BenchmarkLabelJoinLookup10K(b *testing.B) {
	store := benchmarkLabelStore(b)
	const typeKey = "policies.example.test/v1/policies"
	b.ResetTimer()
	for b.Loop() {
		objects, queryErr := store.authorizedObjectsByLabels(b.Context(), "platform", typeKey, "", []labelMatch{{key: "owner", value: "policy-05000"}, {key: "scope", value: "production"}}, 2)
		if queryErr != nil || len(objects) != 1 {
			b.Fatalf("lookup = %d objects, %v", len(objects), queryErr)
		}
	}
}

func BenchmarkLabelJoinLookup10KConcurrent(b *testing.B) {
	store := benchmarkLabelStore(b)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			objects, err := store.authorizedObjectsByLabels(b.Context(), "platform", "policies.example.test/v1/policies", "", []labelMatch{{key: "owner", value: "policy-05000"}, {key: "scope", value: "production"}}, 2)
			if err != nil || len(objects) != 1 {
				b.Fatalf("lookup = %d objects, %v", len(objects), err)
			}
		}
	})
}

func BenchmarkLabelJoinAmbiguity10KLowSelectivity(b *testing.B) {
	store := benchmarkLabelStore(b)
	b.ResetTimer()
	for b.Loop() {
		objects, err := store.authorizedObjectsByLabels(b.Context(), "platform", "policies.example.test/v1/policies", "", []labelMatch{{key: "scope", value: "production"}}, 2)
		if err != nil || len(objects) != 2 {
			b.Fatalf("lookup = %d objects, %v", len(objects), err)
		}
	}
}

func benchmarkLabelStore(b *testing.B) *Projection {
	b.Helper()
	store, err := Open(b.TempDir() + "/projection.db")
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = store.Close() })
	const typeKey = "policies.example.test/v1/policies"
	if syncErr := store.SetSynchronized(b.Context(), typeKey, true); syncErr != nil {
		b.Fatal(syncErr)
	}
	tx, err := store.db.BeginTx(b.Context(), nil)
	if err != nil {
		b.Fatal(err)
	}
	for i := range 10_000 {
		uid := fmt.Sprintf("policy-%05d", i)
		contents := []byte(fmt.Sprintf(`{"metadata":{"uid":%q,"name":%q,"labels":{"owner":%q,"scope":"production"}}}`, uid, uid, uid))
		if _, err = tx.ExecContext(b.Context(), `INSERT INTO resources(type_key,uid,namespace,name,resource_version,contents) VALUES(?,?,?,?,?,?)`, typeKey, uid, "", uid, "1", contents); err != nil {
			b.Fatal(err)
		}
		if _, err = tx.ExecContext(b.Context(), `INSERT INTO resource_tenants(type_key,uid,tenant_id) VALUES(?,?,?)`, typeKey, uid, "platform"); err != nil {
			b.Fatal(err)
		}
		if labelErr := replaceResourceLabels(b.Context(), tx, typeKey, uid, contents); labelErr != nil {
			b.Fatal(labelErr)
		}
	}
	if err = tx.Commit(); err != nil {
		b.Fatal(err)
	}
	return store
}

func TestSnapshotSubscriptionDoesNotLoseCommitDuringSetup(t *testing.T) {
	store, err := Open(t.TempDir() + "/projection.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	const typeKey = "widgets.example.test/v1/widgets"
	if err := store.SetSynchronized(t.Context(), typeKey, true); err != nil {
		t.Fatal(err)
	}
	snapshotReached := make(chan struct{})
	releaseSnapshot := make(chan struct{})
	store.snapshotHook = func() {
		close(snapshotReached)
		<-releaseSnapshot
	}
	type result struct {
		subscription *Subscription
		err          error
	}
	resultCh := make(chan result, 1)
	go func() {
		query := Query{Namespaced: true, Page: 1}
		_, subscription, snapshotErr := store.SnapshotAndSubscribe(context.Background(), "platform", typeKey, &query)
		resultCh <- result{subscription: subscription, err: snapshotErr}
	}()
	<-snapshotReached
	committed := make(chan error, 1)
	go func() {
		contents, _ := json.Marshal(map[string]any{"metadata": map[string]any{"uid": "new-uid", "name": "new"}})
		committed <- store.Replace(context.Background(), typeKey, "new-uid", "", "new", "1", contents, []string{"platform"})
	}()
	close(releaseSnapshot)
	created := <-resultCh
	if created.err != nil {
		t.Fatal(created.err)
	}
	defer created.subscription.Close()
	if err := <-committed; err != nil {
		t.Fatal(err)
	}
	select {
	case _, ok := <-created.subscription.Updates:
		if !ok {
			t.Fatal("subscription closed instead of delivering setup-race invalidation")
		}
	case <-time.After(time.Second):
		t.Fatal("commit during snapshot setup was lost")
	}
}

func TestDetailSubscriptionDoesNotLoseCommitDuringSetup(t *testing.T) {
	store, err := Open(t.TempDir() + "/projection.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	const typeKey = "widgets.example.test/v1/widgets"
	if err := store.SetSynchronized(t.Context(), typeKey, true); err != nil {
		t.Fatal(err)
	}
	contents, _ := json.Marshal(map[string]any{"metadata": map[string]any{"uid": "detail-uid", "name": "detail"}})
	if err := store.Replace(t.Context(), typeKey, "detail-uid", "", "detail", "1", contents, []string{"platform"}); err != nil {
		t.Fatal(err)
	}
	snapshotReached := make(chan struct{})
	releaseSnapshot := make(chan struct{})
	store.snapshotHook = func() {
		close(snapshotReached)
		<-releaseSnapshot
	}
	type result struct {
		subscription *Subscription
		err          error
	}
	resultCh := make(chan result, 1)
	go func() {
		_, subscription, snapshotErr := store.DetailSnapshotAndSubscribe(context.Background(), "platform", typeKey, "detail-uid")
		resultCh <- result{subscription: subscription, err: snapshotErr}
	}()
	<-snapshotReached
	committed := make(chan error, 1)
	go func() {
		committed <- store.Replace(context.Background(), typeKey, "detail-uid", "", "detail", "2", contents, []string{"platform"})
	}()
	close(releaseSnapshot)
	created := <-resultCh
	if created.err != nil {
		t.Fatal(created.err)
	}
	defer created.subscription.Close()
	if err := <-committed; err != nil {
		t.Fatal(err)
	}
	select {
	case _, ok := <-created.subscription.Updates:
		if !ok {
			t.Fatal("detail subscription closed instead of delivering setup-race invalidation")
		}
	case <-time.After(time.Second):
		t.Fatal("commit during detail snapshot setup was lost")
	}
}

func TestPersistedSnapshotIsWithheldUntilAtomicRecoveryReplacesMembership(t *testing.T) {
	path := t.TempDir() + "/projection.db"
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	typeKey := "widgets.example.test/v1/widgets"
	old := ObservedResource{UID: "old", Name: "old", ResourceVersion: "1", Contents: []byte(`{"metadata":{"uid":"old","name":"old"}}`), TenantIDs: []string{"platform"}}
	if err = store.ReplaceSnapshot(t.Context(), typeKey, []ObservedResource{old}); err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err = store.List(t.Context(), "platform", typeKey, &Query{Page: 1}); !errors.Is(err, ErrNotSynchronized) {
		t.Fatalf("persisted stale list error = %v", err)
	}
	current := ObservedResource{UID: "new", Name: "new", ResourceVersion: "2", Contents: []byte(`{"metadata":{"uid":"new","name":"new"}}`), TenantIDs: []string{"payments"}}
	if err = store.ReplaceSnapshot(t.Context(), typeKey, []ObservedResource{current}); err != nil {
		t.Fatal(err)
	}
	platform, err := store.List(t.Context(), "platform", typeKey, &Query{Page: 1})
	if err != nil || platform.Total != 0 {
		t.Fatalf("old membership survived recovery: page=%#v err=%v", platform, err)
	}
	payments, err := store.List(t.Context(), "payments", typeKey, &Query{Page: 1})
	if err != nil || payments.Total != 1 || payments.Items[0].UID != "new" {
		t.Fatalf("replacement snapshot = %#v err=%v", payments, err)
	}
}
