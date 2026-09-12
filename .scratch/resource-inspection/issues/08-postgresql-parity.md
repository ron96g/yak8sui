# 08: Run the same workflows on PostgreSQL

**What to build:** Operators can select PostgreSQL instead of SQLite and users retain the same authorized, live, filtered inspection behavior.

**Blocked by:** 04 — Keep tenant views live and authorized; 07 — Filter by resource JSON values using SQL-first execution.

**Status:** ready-for-agent

## Acceptance criteria

- [ ] Deployment configuration selects SQLite or PostgreSQL and provisions the required latest-state schema through a repeatable initialization/migration mechanism.
- [ ] PostgreSQL persists resource identities, contents, and tenant membership consistently across updates, deletion/recreation, and restart; preserve the combined-process deployment model.
- [ ] Use indexed tenant/type scoping and best-effort native parameterized PostgreSQL JSON predicates with the approved subset. Native regex or JSON lax semantics cannot silently change results relative to Go/SQLite.
- [ ] Both backends return equivalent matching resources and counts for fuzzy, regex, JSONPath, combined filters, and pagination; preserve typed and null/missing distinctions.
- [ ] Run public-interface persistence and query conformance cases against real SQLite and PostgreSQL, including matches beyond early batches and shared/removed tenant membership.
- [ ] With tickets 04 and 07 integrated, live changes refresh matching pages/counts on both backends, including resources entering/leaving filters, while preserving selection by UID.
- [ ] SSE updates reflect committed state and fresh reconnect snapshots on either backend; test changes across snapshot establishment.
- [ ] Inspect representative query plans for indexed scoping and JSON pushdown; document dialect-specific fallback and indexing behavior rather than promising universal index coverage.

## Implementation context

Use ADRs 0007, 0010, and 0011. This ticket adds storage parity, not separate ingestion/API replicas or HA deployment.
