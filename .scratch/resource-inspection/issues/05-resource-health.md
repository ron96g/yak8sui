# 05: Explain current resource health

**What to build:** Resource inspection shows readable conditions and Ready-based health while distinguishing a stale reported generation from an unknown or unhealthy condition.

**Blocked by:** 02 — Inspect a tenant-selected CR end to end.

**Status:** ready-for-agent

## Acceptance criteria

- [ ] Resource responses and UI provide a readable conditions view preserving condition type, status, reason, and message when present; complete YAML remains accessible.
- [ ] The Ready condition maps True to Healthy, False to Unhealthy, and Unknown or absence to Unknown.
- [ ] A condition whose observedGeneration is older than metadata.generation is visibly Stale rather than presented as current confirmation; retain the reported condition evidence.
- [ ] Missing observedGeneration uses the reported condition without asserting generation freshness.
- [ ] Resources with missing/nonstandard status remain inspectable without falsely reporting health; the same Ready rule is reusable for built-in related resources.
- [ ] Public API and focused UI behavior tests cover the Ready mapping, absent conditions, reason/message preservation, and current/old/missing observedGeneration.

## Implementation context

Use ADR 0003. Condition-generation staleness is distinct from observation connection freshness. This ticket can be verified independently of browser live delivery; existing live resource delivery should naturally carry the resulting presentation data when both slices are present.
