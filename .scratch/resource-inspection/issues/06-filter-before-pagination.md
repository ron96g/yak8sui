# 06: Search resource lists before pagination

**What to build:** Tenant users search all selected-type resources by fuzzy or regex Name and Namespace filters and receive correctly counted, stably ordered matching pages.

**Blocked by:** 03 — Discover the configured resource scope.

**Status:** ready-for-agent

## Acceptance criteria

- [ ] Name and Namespace offer case-insensitive ordered-character fuzzy matching and an explicit regex mode using Go regex semantics, case-sensitive by default with inline case-insensitive flags supported.
- [ ] Invalid or unsupported regex patterns produce visible validation errors. Combine active filters with AND; empty filters impose no additional restriction.
- [ ] Cluster-scoped resources ignore namespace filtering, and the UI disables that input using type-scope configuration.
- [ ] Every list query begins with indexed tenant-membership and resource-type scoping in SQLite. Push equivalent predicates into native parameterized SQL on a best-effort basis.
- [ ] Remaining predicates evaluate only scoped candidates. Filter all required candidates before paginating matching results; internal fetching batches never hide later matches.
- [ ] Return stable namespace/name ordering, 100-row pages, and a matching count from the same filtered dataset. Preserve selection by UID.
- [ ] Keep the query/view refresh contract compatible with live delivery: a refresh reruns the same filters and count, and changes entering/leaving the result set can invalidate the view. Ticket 08 verifies integration with ticket 04.
- [ ] Public API tests cover combined filters, Go-specific regex behavior, matches beyond the first candidate batch, empty pages, counts, isolation, and cluster scope; UI tests cover controls and navigation.
- [ ] Inspect representative query plans to verify indexed tenant/type restriction and document which predicates execute natively versus through fallback, without claiming arbitrary regex/fuzzy matching is universally indexed.

## Implementation context

Use ADRs 0002 and 0011. PostgreSQL parity follows in ticket 08. Do not substitute trigram similarity for ordered-character fuzzy matching or database-native regex with different semantics.
