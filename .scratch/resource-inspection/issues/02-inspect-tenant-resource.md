# 02: Inspect a tenant-selected CR end to end

**What to build:** An authorized tenant user can list one configured CR type and inspect a resource's complete current YAML, backed by Kubernetes observation and a SQLite latest-state projection.

**Blocked by:** 01 — Sign in and select an authorized tenant.

**Status:** ready-for-agent

**Implementation note:** Claimed locally on 2026-09-13 for the approved ticket 02 implementation. The tracker status remains the fixed triage role `ready-for-agent`.

## Acceptance criteria

- [x] Observe one explicitly configured CR type with client-go and persist the latest observed resources in SQLite; list, update, and delete observations converge to current state.
- [x] Store enough identity to distinguish resource UIDs and deletion/recreation under the same name; retain complete resource contents including arbitrary status.
- [x] Authorized resource list/detail reads enforce the active tenant's label selector and applicable resource-type restriction in the backend.
- [x] The UI displays a list and a read-only Monaco YAML detail view. Opening a detail does not fetch Kubernetes Events.
- [x] Requests for another tenant's contents are denied or excluded consistently, including direct detail requests bypassing navigation.
- [x] Initial contents are exposed only after observation synchronization; retain a rebuildable latest-state projection rather than resource history. Full restart/failure handling is completed by ticket 09.
- [x] Public HTTP tests with controlled Kubernetes list/watch behavior demonstrate inspection, updates, deletion/recreation, and tenant isolation against SQLite.

## Implementation context

Use ADRs 0001, 0002, 0006, and 0007. Keep ingestion, authorization, and query responsibilities distinct inside the combined process. This slice is deliberately one type; generalized discovery is ticket 03 and live browser updates are ticket 04.

## Comments

- 2026-09-13: Implemented and verified the one-type inspection slice. A client-go dynamic informer observes the configured type, a SQLite latest-state projection stores complete objects and explicit tenant memberships by UID, and synchronized public HTTP list/detail reads enforce tenant and type scope. The browser journey covers sign-in, tenant list, read-only Monaco YAML detail, no Events request, tenant switching, and logout. `go test -race ./...`, `golangci-lint run ./...`, frontend formatting/typecheck/build, and Playwright all passed. Generalized discovery, live delivery, and comprehensive restart/failure UX remain in their later tickets.
