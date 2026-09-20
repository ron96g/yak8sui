# 02: Preview related resource YAML in place

**What to build:** An administrator can select any loaded graph node or table row and inspect its live read-only YAML beside the ownership hierarchy without changing the center resource or losing navigation state.

**Blocked by:** 01 — Navigate the loaded ownership hierarchy as a table.

**Status:** complete

## Acceptance criteria

- [x] Selecting a graph node or table row changes an independent ownership preview and preserves the center resource, presentation mode, loaded branches, and expansion state.
- [x] Reuse the existing read-only YAML viewer in a side preview on desktop and a stacked preview on narrow screens with a usable bounded height.
- [x] Provide an explicit Explore from this resource action that deliberately recenters the hierarchy and retains the prior inspection capability.
- [x] Add an authorized UID-scoped detail replacement stream that delivers complete YAML snapshots after committed changes without relying on the center resource's type/page stream.
- [x] Recheck session, tenant authorization, resource membership, selected type, and observation freshness for preview reads and stream snapshots.
- [x] Show loading, reconnecting, unavailable, revoked, and malformed-response states distinctly; clear or invalidate YAML when it can no longer be asserted current.
- [x] Tenant, center, selection, dialog, and session changes close obsolete preview streams and prevent late responses from replacing the active preview.
- [x] Preserve UID identity across deletion/recreation; a same-named replacement cannot silently become the selected preview.
- [x] Public HTTP/SSE tests cover commit-safe initial delivery, live updates, reconnect, observation loss/recovery, membership removal, session expiry, and deletion/recreation.
- [x] Browser tests cover rapid node comparison in graph and table modes, explicit recentering, live YAML changes, stale-response suppression, keyboard operation, and mobile layout without overflow.

## Implementation context

Use ADRs 0023, 0030, 0033, and 0034. The preview is a second selection within the current ownership context, not a second authorization context or a resource editor.
