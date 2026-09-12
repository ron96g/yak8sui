# 03: Discover the configured resource scope

**What to build:** Operators configure an intentional subset of CR types, and tenant users navigate and inspect all selected types across namespaced and cluster-scoped resources.

**Blocked by:** 02 — Inspect a tenant-selected CR end to end.

**Status:** ready-for-agent

## Acceptance criteria

- [ ] Restart-required YAML selects CRD API groups, explicit type inclusions/exclusions, and optional per-type selectors; exclusions win and type selectors only further restrict tenant selection.
- [ ] Group/type selection and type restrictions are shared across tenants, while tenant membership selectors remain independent.
- [ ] Discover and observe only configured types, supporting both namespaced and cluster-scoped CRs without a namespace-shaped identity assumption.
- [ ] Observe one served version per group/kind, defaulting to the API's preferred served version and honoring valid overrides; the same objects do not appear once per version.
- [ ] The configuration response exposes selected type identity, version, scope, and availability. GroupVersion/Kind navigation reflects this information and reaches list/detail views.
- [ ] Malformed or contradictory configuration fails startup with actionable errors. Valid configuration whose type is temporarily unavailable reports the type unavailable rather than treating the configuration as malformed.
- [ ] Public API tests cover multiple groups, exclusions, selector restrictions, both scopes, version overrides, and unavailable discovery results; a UI check verifies navigation and scope information.

## Implementation context

Use ADRs 0002 and 0012. Basic availability reporting belongs here; comprehensive loss/recovery behavior is ticket 09. Configuration can later include additional relationship resource types without defaulting to whole-cluster observation.
