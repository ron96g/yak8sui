# 03: Discover the configured resource scope

**What to build:** Operators configure an intentional subset of CR types, and tenant users navigate and inspect all selected types across namespaced and cluster-scoped resources.

**Blocked by:** 02 — Inspect a tenant-selected CR end to end.

**Status:** ready-for-agent

**Implementation note:** Claimed locally on 2026-09-13 for the approved ticket
03 implementation. The tracker status remains the fixed triage role
`ready-for-agent`.

## Acceptance criteria

- [x] Restart-required YAML selects CRD API groups, explicit type inclusions/exclusions, and optional per-type selectors; exclusions win and type selectors only further restrict tenant selection.
- [x] Group/type selection and type restrictions are shared across tenants, while tenant membership selectors remain independent.
- [x] Discover and observe only configured types, supporting both namespaced and cluster-scoped CRs without a namespace-shaped identity assumption.
- [x] Observe one served version per group/kind, defaulting to the API's preferred served version and honoring valid overrides; the same objects do not appear once per version.
- [x] The configuration response exposes selected type identity, version, scope, and availability. GroupVersion/Kind navigation reflects this information and reaches list/detail views.
- [x] Malformed or contradictory configuration fails startup with actionable errors. Valid configuration whose type is temporarily unavailable reports the type unavailable rather than treating the configuration as malformed.
- [x] Public API tests cover multiple groups, exclusions, selector restrictions, both scopes, version overrides, and unavailable discovery results; a UI check verifies navigation and scope information.

## Implementation context

Use ADRs 0002 and 0012. Basic availability reporting belongs here; comprehensive loss/recovery behavior is ticket 09. Configuration can later include additional relationship resource types without defaulting to whole-cluster observation.

## Comments

- 2026-09-13: Claimed for implementation. The public seams are the authenticated
  versioned configuration/resource HTTP APIs and the controlled browser fixture.
  Work is limited to configured discovery, independent per-type observation,
  explicit type addressing, and metadata-driven navigation.
- 2026-09-13: Implemented and verified ticket 03. Restart-required group rules
  resolve through Kubernetes discovery into a versioned type catalog; available
  types receive independent informers and unavailable explicit selections remain
  visible without blocking healthy types. Versioned HTTP routes preserve tenant
  and type scoping, and the browser navigates namespaced and cluster-scoped demo
  resources across groups and tenants. Race tests, golangci-lint, frontend
  formatting/typecheck/build, both Playwright suites, and diff checks passed.
  Post-sync outage recovery remains ticket 09.
