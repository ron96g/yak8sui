# 12: Package and verify the complete admin workflow

**What to build:** Operators can deploy the combined application with documented configuration and verify the complete tenant-isolated inspection, ownership, and requested-Events workflow at the agreed provisional scale.

**Blocked by:** 05 — Explain current resource health; 08 — Run the same workflows on PostgreSQL; 09 — Handle observation loss and recovery; 10 — Explore configured ownership chains; 11 — Inspect Events on explicit request.

**Status:** ready-for-agent

## Acceptance criteria

- [ ] Provide a runnable combined ingestion/API deployment serving the frontend from the same origin, with documented OIDC, tenant selectors, group/type/version selection, hierarchy configuration, and SQLite/PostgreSQL choices.
- [ ] Supply practical configuration examples and Kubernetes permissions for configured observation and explicitly requested Events, including both namespaced and cluster-scoped resources.
- [ ] Run the integrated HTTP/SSE acceptance suite against both database adapters, including restart/outage recovery and ownership cases introduced after PostgreSQL parity.
- [ ] Run a compact browser journey for login, tenant switching, state/health inspection, all filters before pagination, live updates/reconnect, hierarchy traversal, explicit Events, and logout.
- [ ] Integrated checks verify that unauthorized resource contents never appear in lists, details, streams, hierarchy responses, or Events; exercise labels changing during observation loss and late responses during tenant switching.
- [ ] Live filtered pages and matching counts remain correct when resources enter/leave filters or tenants; health and hierarchy views reflect updates while preserving UID identity.
- [ ] Reproduce a workload with 10,000 observed resources, 20 tenants, and 20 simultaneous viewers. Measure ordinary update visibility against the two-second target from backend observation.
- [ ] Record environment, selected workload/update rate, latency measurements, observed failures, and query-plan evidence for indexed scoping/native SQL. Investigate target failures; distinguish measured results from untested production SLAs or arbitrary-query guarantees.
- [ ] Document deployment, startup validation, restart-required configuration changes, unavailable-type troubleshooting, how to run verification, and known limitations of configured hierarchy completeness and scoped query fallback.

## Implementation context

Use the complete approved spec and ADRs 0001–0014. Keep this slice focused on assembling, packaging, and validating delivered behavior; broader multi-cluster, HA, mutation, history, and agent-driven diagnosis remain deferred.
