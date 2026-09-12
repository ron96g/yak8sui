# 10: Explore configured ownership chains

**What to build:** Tenant users navigate a selected resource's authorized parents and descendants through explicitly configured ownership relationships, including configured built-in resource types.

**Blocked by:** 03 — Discover the configured resource scope; 04 — Keep tenant views live and authorized; 09 — Handle observation loss and recovery.

**Status:** ready-for-agent

## Acceptance criteria

- [ ] Validated YAML declares permitted parent/child type relationships by API group and kind and the additional relationship resource types to observe; respect existing exclusions and tenant/type restrictions.
- [ ] Actual edges require matching ownerReferences resolved by UID. A type rule or matching object name alone never establishes ownership.
- [ ] Both parent and child traversal work for CRs and configured built-in types, including multiple parents and controller-owner markers.
- [ ] Each traversed resource independently satisfies the active tenant selector; unlabeled/inaccessible intermediates stop traversal without fetched contents being exposed or access inherited through ownership.
- [ ] The UI centers on the selected resource with parents above and children below, loads branches on demand, guards against cycles, and permits navigation through the entire configured authorized chain.
- [ ] Unavailable or out-of-observation-scope branches indicate limited completeness rather than asserting that no relationships exist; such indicators do not expose inaccessible fetched resource contents.
- [ ] Live ownership changes, deletions, recreation, and membership changes refresh or invalidate expanded branches without leaving unauthorized related details accessible.
- [ ] Introduce a relationship resolver boundary permitting future per-relationship YAML resolver selection; only owner-reference resolution is implemented now.
- [ ] Public API/SSE and browser cases cover upward/downward traversal, built-in children, same-name/different-UID objects, multiple parents, cycles, missing labels, unavailable types, and branch expansion. Persistence-sensitive cases run on available adapters and both in ticket 12.

## Implementation context

Use ADRs 0002, 0004, and 0012. Configuration establishes permitted type edges; future automatic hierarchy discovery and label/spec/status resolver implementations are outside this ticket.
