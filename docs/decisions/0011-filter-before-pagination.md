# Apply every filter before pagination

Filtering searches all authorized, observed resources of the selected type on the backend. Apply every filter before pagination, using stable namespace/name ordering and 100-row pages initially. Live changes refresh the matching page and count; preserve resource selection by UID.

Use best-effort native SQL execution and indexes while preserving the agreed matching semantics. Restrict candidates by tenant membership and resource type using indexed SQL first, and push additional predicates into SQL wherever their semantics match exactly.

When a predicate cannot use an index or cannot be expressed faithfully in native SQL, permit evaluation over the scoped candidates using existing libraries where suitable. Apply pagination only to matching results and calculate counts from the same filtered dataset. Exact counts may require evaluating all scoped candidates; universally index-backed execution is not a requirement.

Add targeted indexes for frequently queried JSON paths as workloads become clear. Avoid unnecessary whole-cluster scans without changing fuzzy, Go-regex, or JSONPath semantics to fit a database optimization.
