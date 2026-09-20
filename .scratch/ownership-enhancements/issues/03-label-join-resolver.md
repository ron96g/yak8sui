# 03: Resolve logical ownership through composite labels

**What to build:** Operators can configure a parent/child type pair to resolve ownership from one or more matching parent and child labels, and tenant users can traverse the resulting authorized live hierarchy in both directions.

**Blocked by:** None (can start immediately).

**Status:** complete

## Acceptance criteria

- [x] Extend restart-required relationship configuration so each directional type pair selects exactly one resolver; omitted resolver remains `ownerReferences`, and `labelJoin` accepts a non-empty list of parent-label/child-label key mappings.
- [x] Reject duplicate type pairs, duplicate or empty mappings, missing label-join options, options belonging to other resolvers, unknown resolver names, and relationship endpoints outside configured observation.
- [x] Dispatch relationship rules through resolver adapters while preserving one tenant-authorized immediate-neighborhood HTTP/SSE contract.
- [x] Create a label edge only when every configured endpoint label is present and all mapped values are equal; label keys may differ between parent and child.
- [x] Allow one parent to own multiple children and enforce at most one matching parent for each custom-resolved child.
- [x] If multiple authorized current parents match, return no arbitrary parent and mark the affected direction incomplete with a non-sensitive ambiguity diagnostic.
- [x] Treat zero matches under a healthy rule as authoritative absence, not an error.
- [x] Dynamically rebind logical ownership when a parent is replaced by another current authorized parent with the same joined labels; report the replacement parent's actual UID and metadata.
- [x] Independently enforce tenant membership, type selection, observation freshness, and namespace/cluster scope for every endpoint; ownership never grants access or crosses an inaccessible intermediary.
- [x] Preserve existing owner-reference behavior, including multiple parents and controller markers, and support owner-reference and label-join rules for different type pairs in one configuration.
- [x] Public HTTP/SSE and configuration tests cover composite AND matching, differing keys, missing values, multiple children, ambiguity, recreation/rebinding, upward/downward traversal, live updates, unavailable types, scope rejection, and tenant isolation.
- [x] Update example configuration, README resolver documentation, controlled demo data, and browser coverage with a label-joined hierarchy branch.

## Comments

Implemented composite label joins through a transactionally maintained, indexed SQLite label projection. Parent ambiguity uses bounded SQL lookup, child traversal uses SQL label predicates, and public HTTP/SSE plus controlled-browser coverage exercise mixed resolvers and live updates. See ADRs 0037 and 0038.

## Implementation context

Use ADRs 0004, 0030, and 0035. Label joins represent logical ownership and therefore intentionally differ from owner-reference UID continuity.
