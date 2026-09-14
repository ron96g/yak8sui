# Recover observation freshness through a new authoritative relist

Track observation freshness independently for each configured group/version/resource
as `current`, `stale`, `resynchronizing`, or `unavailable`, with an actionable
reason. Persist that status alongside the SQLite latest-state projection. Every
new list, detail, and stream snapshot remains gated unless the type is current;
an existing stream receives an authorized status-only event and closes when its
type loses freshness, retaining already-rendered contents only as visibly stale.

Treat client-go's `HasSynced` as an initial-attempt barrier, not enduring proof
of freshness. Recovery creates a new informer, waits for its initial list, and
atomically replaces the complete type projection—including tenant memberships
and deletions—before marking it current and publishing an invalidation. Projection
write failures also revoke freshness. Expected watch timeout closure may reconnect
inside the same reflector without revoking freshness; explicit list/watch errors,
authorization failures, missing resources, and controlled observer interruption
end the attempt and enter bounded exponential retry. This is deliberately
conservative: a transient successful watch reconnect does not promote stale data.

Retry rediscovery is scoped by the restart-required configured group/resource
selection. Explicitly configured absent types can return, but a selected served
version is pinned for the process lifetime: another served version does not
silently replace it or create a duplicate observer. Configuration hot reload,
automatic ownership discovery, PostgreSQL, and HA coordination remain out of
scope.
