# 11: Inspect Events on explicit request

**What to build:** A tenant user opens a resource's Events tab to fetch current UID-scoped Kubernetes Events and manually refreshes when more evidence is needed.

**Blocked by:** 02 — Inspect a tenant-selected CR end to end.

**Status:** ready-for-agent

## Acceptance criteria

- [ ] Resource lists and ordinary detail opening do not fetch or ingest Kubernetes Events. Fetch only on Events-tab activation or explicit manual refresh.
- [ ] The backend rechecks session, tenant authorization, resource membership, and resource availability before requesting or returning Events.
- [ ] Requests correlate Events to the selected resource UID, not just name, so deletion/recreation cannot mix evidence from different objects.
- [ ] The UI displays loading, successful empty results, available Events, and retrieval failure distinctly; it provides a manual refresh action without polling or continuous subscription.
- [ ] Tenant switching, resource switching, or access loss cannot render a late Event response in the wrong context.
- [ ] Public HTTP tests verify UID correlation and access checks. Controlled Kubernetes requests demonstrate no Events fetch before activation and exactly user-triggered fetching afterward.
- [ ] A browser scenario verifies tab activation, manual refresh, errors, and recreated-resource isolation. Honor the availability contract from ticket 09 when integrated, as verified in ticket 12.

## Implementation context

Use ADRs 0001, 0002, and 0005. This is evidence presentation rather than automated diagnosis, Event history storage, or a live Event stream.
