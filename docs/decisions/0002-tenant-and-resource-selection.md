# Select observed Custom Resources by API group and tenant labels

YAK8SUI initially runs against the cluster in which it is deployed and must support multiple tenants within that cluster, with tenant membership selected by labels on CRs. Both namespaced and cluster-scoped CRs are supported, with resource-name and namespace filtering in the UI.

Discovery selects CRD API groups, such as `cert-manager.io`, with optional overrides rather than monitoring all CRs in the cluster. Deployment configuration supplies group inclusions, explicit resource-type inclusions/exclusions, and optional per-type selectors. Exclusions win, and per-type selectors further restrict selection without relaxing tenant boundaries.

Tenants are a security boundary enforced by the backend across resource lists, details, live updates, Events, and hierarchy traversal. Multiple configured tenants are required in the first increment. Use OIDC identity with deployment configuration mapping identity-provider groups to tenants. Users may switch among their authorized tenants; the backend independently enforces authorization for every request.

Each resource must independently satisfy the tenant label selector; ownership does not grant access to otherwise inaccessible resources. Overlapping selectors intentionally grant shared visibility. Label changes remove resources from views of tenants they no longer match. Group/type selections, per-type restrictions, and hierarchy rules are shared initially; tenant selectors vary by tenant.

Both resource-name and namespace filters support case-insensitive ordered-character fuzzy matching and an explicit Regex mode in the first increment. Invalid regex patterns produce validation errors. Combine Name and Namespace filters with AND. Cluster-scoped resources ignore the namespace filter. The configuration API exposes resource scope so the UI can disable the namespace input for cluster-scoped types.

Observe one served version per group/kind, defaulting to the API's preferred version with a configuration override. Show the selected version in navigation and details without duplicating objects across versions.

Deployment YAML changes require a restart initially. Validate configuration at startup and reject invalid or contradictory rules with actionable errors.

Regex mode uses Go's standard regular expression semantics, case-sensitive by default with `(?i)` available for case-insensitive matching. Invalid or unsupported patterns produce validation errors.
