# 02: Inspect a tenant-selected CR end to end

**What to build:** An authorized tenant user can list one configured CR type and inspect a resource's complete current YAML, backed by Kubernetes observation and a SQLite latest-state projection.

**Blocked by:** 01 — Sign in and select an authorized tenant.

**Status:** ready-for-agent

## Acceptance criteria

- [ ] Observe one explicitly configured CR type with client-go and persist the latest observed resources in SQLite; list, update, and delete observations converge to current state.
- [ ] Store enough identity to distinguish resource UIDs and deletion/recreation under the same name; retain complete resource contents including arbitrary status.
- [ ] Authorized resource list/detail reads enforce the active tenant's label selector and applicable resource-type restriction in the backend.
- [ ] The UI displays a list and a read-only Monaco YAML detail view. Opening a detail does not fetch Kubernetes Events.
- [ ] Requests for another tenant's contents are denied or excluded consistently, including direct detail requests bypassing navigation.
- [ ] Initial contents are exposed only after observation synchronization; retain a rebuildable latest-state projection rather than resource history. Full restart/failure handling is completed by ticket 09.
- [ ] Public HTTP tests with controlled Kubernetes list/watch behavior demonstrate inspection, updates, deletion/recreation, and tenant isolation against SQLite.

## Implementation context

Use ADRs 0001, 0002, 0006, and 0007. Keep ingestion, authorization, and query responsibilities distinct inside the combined process. This slice is deliberately one type; generalized discovery is ticket 03 and live browser updates are ticket 04.
