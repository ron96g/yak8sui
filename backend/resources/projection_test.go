package resources

import (
	"context"
	"encoding/json"
	"errors"
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
