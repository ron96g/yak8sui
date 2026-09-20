# 04: Resolve ownership through structured child fields

**What to build:** Operators can configure one child spec/status path containing a structured parent reference, and tenant users can traverse the resulting UID-stable authorized ownership edge in both directions.

**Blocked by:** 03 — Resolve logical ownership through composite labels.

**Status:** ready-for-agent

## Acceptance criteria

- [ ] Add `fieldReference` resolver configuration with one validated path rooted under child `spec` or `status`; reject paths elsewhere, absent paths, arrays, and options belonging to another resolver.
- [ ] Interpret the selected value as one ObjectReference subset containing API version, kind, name, UID, and namespace when required by resource scope.
- [ ] Resolve an edge only when reference group, served version, kind, name, UID, namespace, and valid parent/child scope identify the same authorized current configured parent.
- [ ] Preserve UID-stable behavior: deleting and recreating a parent under the same name does not rebind an old field reference.
- [ ] Apply the custom-resolver one-parent invariant. A malformed reference or ambiguous resolution produces no arbitrary edge and marks the affected direction incomplete with a safe diagnostic.
- [ ] Treat a missing reference under an otherwise healthy rule as authoritative absence while distinguishing malformed present data from absence.
- [ ] Support paths in both spec and status, core and non-core parent types, and upward/downward traversal through the same neighborhood HTTP/SSE interface.
- [ ] Independently enforce tenant membership, explicit observation scope, freshness, and Kubernetes namespace/cluster scope without exposing inaccessible candidate details.
- [ ] Run owner-reference, label-join, and field-reference rules for different type pairs in the same process and preserve deterministic neighborhood ordering and live invalidation.
- [ ] Public HTTP/SSE and configuration tests cover valid spec/status references, missing and malformed values, type/version/name/UID/namespace mismatches, parent recreation, unavailable types, live reference changes, scope rejection, and tenant isolation.
- [ ] Update example configuration, README resolver documentation, controlled demo data, and browser coverage with a field-referenced hierarchy branch visible in graph and table presentations.
- [ ] Run the complete ownership browser journey with live YAML preview across all three resolver types and document remaining resolver limitations.

## Implementation context

Use ADRs 0004, 0030, and 0035. This ticket builds on the resolver dispatch and custom completeness semantics established by ticket 03; arbitrary field expressions and arrays remain outside scope.
