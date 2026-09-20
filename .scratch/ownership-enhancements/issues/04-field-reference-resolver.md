# 04: Resolve ownership through structured child fields

**What to build:** Operators can configure one child spec/status path containing a structured parent reference, and tenant users can traverse the resulting UID-stable authorized ownership edge in both directions.

**Blocked by:** 03 — Resolve logical ownership through composite labels.

**Status:** resolved

## Acceptance criteria

- [x] Add `fieldReference` resolver configuration with one validated path rooted under child `spec` or `status`; reject paths elsewhere, absent paths, arrays, and options belonging to another resolver.
- [x] Interpret the selected value as one ObjectReference subset containing API version, kind, name, UID, and namespace when required by resource scope.
- [x] Resolve an edge only when reference group, served version, kind, name, UID, namespace, and valid parent/child scope identify the same authorized current configured parent.
- [x] Preserve UID-stable behavior: deleting and recreating a parent under the same name does not rebind an old field reference.
- [x] Apply the custom-resolver one-parent invariant. A malformed reference or ambiguous resolution produces no arbitrary edge and marks the affected direction incomplete with a safe diagnostic.
- [x] Treat a missing reference under an otherwise healthy rule as authoritative absence while distinguishing malformed present data from absence.
- [x] Support paths in both spec and status, core and non-core parent types, and upward/downward traversal through the same neighborhood HTTP/SSE interface.
- [x] Independently enforce tenant membership, explicit observation scope, freshness, and Kubernetes namespace/cluster scope without exposing inaccessible candidate details.
- [x] Run owner-reference, label-join, and field-reference rules for different type pairs in the same process and preserve deterministic neighborhood ordering and live invalidation.
- [x] Public HTTP/SSE and configuration tests cover valid spec/status references, missing and malformed values, type/version/name/UID/namespace mismatches, parent recreation, unavailable types, live reference changes, scope rejection, and tenant isolation.
- [x] Update example configuration, README resolver documentation, controlled demo data, and browser coverage with a field-referenced hierarchy branch visible in graph and table presentations.
- [x] Run the complete ownership browser journey with live YAML preview across all three resolver types and document remaining resolver limitations.

## Implementation context

Use ADRs 0004, 0030, and 0035. This ticket builds on the resolver dispatch and custom completeness semantics established by ticket 03; arbitrary field expressions and arrays remain outside scope.

Before adding the third resolver, deepen the existing ownership module with a
closed internal adapter seam for `ownerReferences`, `labelJoin`, and
`fieldReference`. Keep rule selection, authorized/current reads, scope checks,
directional completeness, safe diagnostics, deduplication, ordering, and live
invalidation in shared orchestration. Keep extraction, identity semantics, and
lookup strategy inside each resolver adapter; do not introduce a generic query
engine or externally extensible plugin registry.

Normalize every `metadata.ownerReferences` entry into an indexed SQLite table,
maintained transactionally with its child resource. Do not restrict ownership to
the single controller reference: non-controller references remain valid edges,
multiple parents remain supported, and the controller flag remains presentation
metadata. Downward owner-reference traversal must query this index and then join
tenant membership and current resources; a projected edge never grants access.
Design the table for all references from the outset rather than introducing a
controller-only column or a temporary controller fast path. See ADR 0039.

Field references remain configuration-dependent and are not stored in the
owner-reference table. Resolve upward field references by exact authorized parent
UID lookup. For downward field-reference traversal, first use a parameterized
SQLite JSON query over the tenant/type-scoped child set so nonmatching resource
documents are not transferred to Go. This remains linear in authorized children;
benchmark the no-match, one-match, many-match, and concurrent cases at the
10,000-resource provisional target before deciding whether a separate extracted
field-reference index is justified.

## Comments

Implemented the structured field-reference resolver and the ADR 0039 all-owner-
reference projection. Downward field-reference lookup remains an indexed tenant/
type-scoped SQLite JSON traversal; the 10,000-resource no-match, one-match,
many-match, and concurrent benchmarks did not justify a separate extracted field
index in this increment.

Verified with `go test ./...`, `golangci-lint run ./...`, frontend formatting,
typechecking, production build, and the Playwright browser suite. The separate
`test:demo` command remains blocked by an unrelated existing process listening on
port 19080; the equivalent controlled browser journey passed via `test:browser`.
