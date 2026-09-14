# Project tenant memberships with resources in SQLite

For the first explicitly configured resource type, observe Kubernetes through a
client-go dynamic shared informer and write complete unstructured JSON into a
SQLite latest-state projection. Identify rows by configured type and Kubernetes
UID, with a unique type/namespace/name index so deletion and same-name
recreation replace identity rather than retaining history.

Materialize each resource's independently evaluated tenant memberships in a
join table. Public list and UID-detail queries must join through tenant ID and
configured type before returning contents; the configured type selector is
applied by the informer and cannot widen tenant membership. Mark the type
unsynchronized before observation, reconcile persisted rows against the first
authoritative informer snapshot, then expose reads. Persist JSON losslessly and
render YAML only at the API boundary.

Use a configurable SQLite path and standard client-go in-cluster configuration,
with an optional kubeconfig path for local operation. Generalized discovery,
live delivery, and comprehensive degraded/restart UX remain later tickets.
