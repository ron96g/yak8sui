# 06: Search resource lists before pagination

**What to build:** Tenant users search all selected-type resources by fuzzy or regex Name and Namespace filters and receive correctly counted, stably ordered matching pages.

**Blocked by:** 03 — Discover the configured resource scope.

**Status:** ready-for-agent

## Acceptance criteria

- [x] Name and Namespace offer case-insensitive ordered-character fuzzy matching and an explicit regex mode using Go regex semantics, case-sensitive by default with inline case-insensitive flags supported.
- [x] Invalid or unsupported regex patterns produce visible validation errors. Combine active filters with AND; empty filters impose no additional restriction.
- [x] Cluster-scoped resources ignore namespace filtering, and the UI disables that input using type-scope configuration.
- [x] Every list query begins with indexed tenant-membership and resource-type scoping in SQLite. Push equivalent predicates into native parameterized SQL on a best-effort basis.
- [x] Remaining predicates evaluate only scoped candidates. Filter all required candidates before paginating matching results; internal fetching batches never hide later matches.
- [x] Return stable namespace/name ordering, 100-row pages, and a matching count from the same filtered dataset. Preserve selection by UID.
- [x] Keep the query/view refresh contract compatible with live delivery: a refresh reruns the same filters and count, and changes entering/leaving the result set can invalidate the view. Ticket 08 verifies integration with ticket 04.
- [x] Public API tests cover combined filters, Go-specific regex behavior, matches beyond the first candidate batch, empty pages, counts, isolation, and cluster scope; UI tests cover controls and navigation.
- [x] Inspect representative query plans to verify indexed tenant/type restriction and document which predicates execute natively versus through fallback, without claiming arbitrary regex/fuzzy matching is universally indexed.

## Implementation context

Use ADRs 0002 and 0011. PostgreSQL parity follows in ticket 08. Do not substitute trigram similarity for ordered-character fuzzy matching or database-native regex with different semantics.

## Comments

### 2026-09-13 — Implemented and verified

Implemented the ticket 06 vertical slice and recorded the query/live contract in
ADR 0026. Name and Namespace now support explicit fuzzy and Go-regex modes;
authorized tenant/type candidates are selected by indexed SQLite access before
Go-semantic matching, counting, stable ordering, and 100-row pagination. REST
and query-keyed SSE return the same page/count shape and rerun it on committed
invalidations. Cluster-scoped namespace predicates are ignored and disabled in
the UI. UID detail remains open when its row moves off-page, but tenant-membership
loss or deletion yields detail 404 and closes it.

Public HTTP tests cover combined Unicode-capable subsequence matching, Go regex,
invalid modes/patterns, no matches, more than two pages of candidates, ordering,
counts, tenant isolation, cluster scope, filtered live insertion/removal, and
off-page versus unauthorized detail. The browser suite covers mode controls,
validation, pagination, scope disabling, and live filter entry/exit. SQLite
`EXPLAIN QUERY PLAN` confirms `resource_tenant_scope` and the resources primary
key are used for initial scoping. `go test -race ./...`, `golangci-lint run
./...`, frontend formatting/typecheck/build, both Playwright suites, and React
Doctor were run; required checks pass. React Doctor reports the existing large
component/effect/fetch/finally heuristics and lazy Monaco import internals, with
no newly identified correctness issue. No ticket 07 JSONPath behavior was added.

### 2026-09-13 — Search-as-you-type UX follow-up

The filter controls now submit the latest combined Name, Namespace, and mode
draft after a 300ms pause, with no Apply action. Filter edits reset to page one;
pagination remains immediate. Pending drafts survive authorized SSE snapshots,
while tenant/type changes and unmount cancel deferred submissions. Browser
coverage verifies debounce coalescing, clear and mode changes, invalid-to-valid
regex recovery, page reset, selected-type refresh, type-switch cancellation,
and an SSE update during pending input. This remains ticket 06 filter behavior;
JSONPath is still disabled and ticket 07 was not implemented.
