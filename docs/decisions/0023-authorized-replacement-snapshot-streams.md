# Deliver scoped replacement snapshots from commit invalidations

Use one SSE stream per authorized tenant and explicit group/version/resource.
The SQLite projection publishes type-level invalidations only after commits; the
stream independently reruns the tenant/type-scoped query and sends a complete
replacement snapshot. This keeps authorization at the query boundary and lets
later filtered, paginated, and hierarchy views consume the same invalidation
mechanism without broadcasting unfiltered resource payloads.

Register a bounded subscription while holding the same publication lock used by
commits, then create the initial scoped snapshot before releasing that lock. A
commit is therefore represented either by the initial snapshot or by a later
invalidation. Each subscriber has a one-item buffer. If another commit arrives
before it consumes the pending invalidation, close that stream so native
`EventSource` reconnects, reauthorizes, and receives a fresh replacement snapshot;
ingestion never blocks and changes are never silently dropped into an inconsistent
incremental view.

Bind every stream to both request cancellation and the existing server-managed
session cancellation signal. Browser reconnect is a delivery state and is distinct
from upstream observer availability; comprehensive observer outage recovery remains
ticket 09.
