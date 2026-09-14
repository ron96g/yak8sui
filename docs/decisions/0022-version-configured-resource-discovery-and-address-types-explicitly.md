# Version configured discovery and address resource types explicitly

YAK8SUI resolves restart-required API-group rules against Kubernetes discovery
into a catalog containing one group/version/resource/kind and scope per selected
type. An empty group `include` selects all currently discovered resources in
that group; a non-empty list also preserves unavailable entries so operators can
see temporary absence. `exclude` always wins. Per-resource version and label
selector overrides are shared across tenants, and the selector is applied by
the informer in addition to independently evaluated tenant selectors.

The default version is the API group's preferred served version when that
resource exists there, otherwise the first discovered served version for the
resource. A configured version override must be served or the selected type is
reported unavailable. Discovery facts determine kind and namespaced or cluster
scope; deployment YAML does not duplicate those facts.

Expose this catalog at authenticated `GET /api/v1/configuration`. Address list
and UID detail reads with the full group/version/resource tuple under
`/api/v1/tenants/{tenant}/resources/...`. This removes the ambiguous pre-release
single-type routes and prevents collisions between identical kinds or UIDs in
different API groups or versions. Every resource query still authorizes the
tenant and scopes SQL by tenant plus the catalog-selected type key.

Start an independent informer for every available catalog entry. Unavailable
entries start no observer, remain visible in configuration, and return 503 from
their resource route without blocking healthy types. Comprehensive
post-synchronization loss and rediscovery remain ticket 09.
