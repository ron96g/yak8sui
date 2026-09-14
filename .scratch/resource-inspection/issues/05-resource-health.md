# 05: Explain current resource health

**What to build:** Resource inspection shows readable conditions and Ready-based health while distinguishing a stale reported generation from an unknown or unhealthy condition.

**Blocked by:** 02 — Inspect a tenant-selected CR end to end.

**Status:** ready-for-agent

## Acceptance criteria

- [x] Resource responses and UI provide a readable conditions view preserving condition type, status, reason, and message when present; complete YAML remains accessible.
- [x] The Ready condition maps True to Healthy, False to Unhealthy, and Unknown or absence to Unknown.
- [x] A condition whose observedGeneration is older than metadata.generation is visibly Stale rather than presented as current confirmation; retain the reported condition evidence.
- [x] Missing observedGeneration uses the reported condition without asserting generation freshness.
- [x] Resources with missing/nonstandard status remain inspectable without falsely reporting health; the same Ready rule is reusable for built-in related resources.
- [x] Public API and focused UI behavior tests cover the Ready mapping, absent conditions, reason/message preservation, and current/old/missing observedGeneration.

## Implementation context

Use ADR 0003. Condition-generation staleness is distinct from observation connection freshness. This ticket can be verified independently of browser live delivery; existing live resource delivery should naturally carry the resulting presentation data when both slices are present.

## Comments

- Implementation claimed 2026-09-13. Public HTTP/SSE responses and focused browser behavior are the approved seams; ADR 0024 records the response and stale-presentation clarification.
- Implemented and verified 2026-09-13. List, detail, and replacement-snapshot responses now share Ready-based health and readable condition evidence; stale Ready reports retain their Healthy/Unhealthy mapping but are prominently labeled Stale and explicitly denied as current confirmation. The table and modal update through the existing scoped SSE lifecycle, full read-only YAML remains available, and the controlled demo cycles healthy to stale to unhealthy for live verification. Ticket 06 filtering and later inspection features remain out of scope.
