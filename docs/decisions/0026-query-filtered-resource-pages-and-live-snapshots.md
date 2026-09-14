# Query filtered resource pages and live snapshots

Resource list and SSE endpoints share one versioned query contract: `name`,
`namespace`, `nameMode`, `namespaceMode`, and one-based `page`. Modes are
`fuzzy` (the default) or `regex`; pages contain at most 100 rows and return
`items`, `total`, `page`, `pageSize`, and `pageCount`. The contract intentionally
allows ticket 07 to add validated JSONPath fields without creating a second
query path.

SQLite first restricts candidates through the materialized
`resource_tenants(tenant_id,type_key,uid)` index and joins resources by its
primary key. Name and namespace matching then runs over only those authorized,
type-scoped candidates before count and pagination. Ordered-character fuzzy
matching uses Unicode lower-cased runes. Regex is compiled and evaluated by
Go's `regexp` package so SQLite regex dialect differences cannot alter behavior;
invalid expressions produce field-specific HTTP 400 errors. Empty predicates
match everything, predicates combine with AND, and namespace predicates are not
compiled or evaluated for cluster-scoped types.

This scoped fallback intentionally scans all candidates for exact counts; it
does not promise index-backed arbitrary fuzzy or regex matching. Representative
`EXPLAIN QUERY PLAN` output uses `resource_tenant_scope (tenant_id,type_key)`
followed by the resources primary-key lookup, rather than a whole-cluster scan.
Future exact native predicates may be parameterized and pushed into SQL only
when their semantics are identical.

Each query owns its SSE subscription. Invalidation reruns that same query and
emits a replacement page and count. The browser closes and rekeys subscriptions
on filter, page, tenant, or type changes, and revision guards reject obsolete
responses. A selected UID is independent of page membership: snapshots refresh
its tenant-authorized detail even when filtering or pagination moves it off the
page; a detail 404 caused by deletion or membership loss closes the selection.
