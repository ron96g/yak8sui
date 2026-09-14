# 09: Handle observation loss and recovery

**What to build:** Users can distinguish current, stale, resynchronizing, and unavailable observation, while unaffected resource types continue to work and stale tenant membership cannot expose contents.

**Blocked by:** 04 — Keep tenant views live and authorized.

**Status:** resolved

## Acceptance criteria

- [ ] Persist last observed resource state across backend restart but withhold contents on new reads until the relevant observation scope synchronizes.
- [ ] When observation is lost, affected open views visibly become stale and suspend updates; new list/detail reads do not serve unsynchronized cached contents.
- [ ] A failing resource type does not block synchronized types; missing CRDs, denied observation, and connection loss produce explicit availability information rather than authoritative empty lists.
- [ ] Browser reconnect and backend resynchronization revalidate access before publishing a fresh view. Labels changed during downtime are reflected before visibility resumes.
- [ ] Recovery reconciles removed/recreated objects and establishes a gap-free fresh snapshot/update transition without exposing stale membership.
- [ ] Public HTTP/SSE tests cover restart with persisted state, label changes during outage, type-specific failures, healthy-type availability, recovery, and permission changes. Run against each available database adapter; ticket 12 verifies both after all branches merge.
- [ ] A browser scenario demonstrates unavailable and resynchronizing states and recovery; keep condition-generation staleness distinct from observation freshness.

## Implementation context

Use ADRs 0007, 0010, and 0012. Observation availability is also consumed by later hierarchy branches. PostgreSQL is not a prerequisite: implement against the shared persistence contract and existing SQLite slice first if necessary.

## Comments

Implemented for SQLite in ADR 0029. Observation is tracked per configured type;
new reads are gated unless current, open streams receive an authorized status
event and close, and recovery creates a fresh informer and atomically replaces
the complete type projection after its initial list. Bounded retries perform
targeted configured-type rediscovery while preserving the selected version.
Public HTTP/SSE and browser coverage exercise stale membership replacement and
partial-type continuity. Ticket 08 PostgreSQL remains explicitly deferred.

**Answer:** Ticket 09 is complete for the current SQLite adapter. See
`docs/decisions/0029-relist-to-recover-observation-freshness.md` for the freshness
boundary, atomic recovery, SSE status, retry, rediscovery, and version-pinning
decisions.
