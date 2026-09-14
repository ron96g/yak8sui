# Resolve authorized ownership as live immediate neighborhoods

Declare directed parent/child type pairs under restart-required
`observation.relationships`. Types are identified by Kubernetes API group and
kind; the empty group is the real core API group. Both endpoint types must also
be explicitly selected under `observation.groups`, so a relationship never
broadens observation. An omitted resolver means `ownerReferences`; reject every
other resolver until it is implemented behind the relationship resolver
interface.

A configured pair permits resolution but does not create an edge. Resolve an
edge only when a child `metadata.ownerReferences` entry has the parent's exact
UID, kind, and parsed `apiVersion` group and selected served version. Ignore the
reference name for identity. Reject impossible scope combinations: a
cluster-scoped child cannot have a namespaced owner, and namespaced parent and
child objects must share a namespace.

Expose a tenant-authorized immediate-neighborhood HTTP API and replacement SSE
stream. Every center, parent candidate, and child candidate is read through
materialized tenant membership and current-observation gates. Never cross an
inaccessible intermediary. Changes committed by any observed type invalidate
bounded hierarchy streams, which rerun the scoped resolver; this favors secure,
simple invalidation over maintaining a second relationship cache. Clients expand
one neighborhood at a time and guard repeated type/UID identities as cycles.

Completeness is reported per direction. A healthy configured scope can state an
authoritative empty neighborhood. Unavailable configured scopes mark that
direction incomplete without exposing object names or counts. A direction with
no configured rules is configuration-limited and makes no claim about the whole
cluster graph.
