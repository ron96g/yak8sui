# 04: Keep tenant views live and authorized

**What to build:** Tenant users see resource changes automatically through authorized SSE snapshots and updates, including removal when membership changes and clean recovery after browser reconnects.

**Blocked by:** 03 — Discover the configured resource scope.

**Status:** ready-for-agent

## Acceptance criteria

- [x] Native EventSource or a suitable existing client/protocol implementation consumes an authorized initial snapshot followed by resource updates.
- [x] The snapshot-to-update handoff does not lose changes arriving during connection setup; updates correspond to committed projection state.
- [x] Browser disconnect/reconnect displays connection/resynchronization state, reauthorizes the tenant, and replaces the view with a fresh snapshot rather than replaying history.
- [x] Label changes remove resources from tenants they no longer match, including an open detail; shared-selector resources are visible in each authorized matching tenant.
- [x] Tenant switching clears the previous tenant's view and subscriptions; updates from a prior tenant cannot populate the new view.
- [x] Logout and session expiry close streams and clear displayed tenant data. A stream cannot outlive its authorized session.
- [x] Selection follows UID; deleting and recreating a same-named object cannot silently substitute it as the old selected object.
- [x] HTTP/SSE tests exercise concurrent snapshot changes, multiple tenants, overlap, membership removal, reconnect, and expiry; a browser check demonstrates automatic updates.

## Implementation context

Use ADRs 0002, 0003, and 0010. Keep live invalidation usable by subsequent paginated-query and hierarchy views. Database observation failure handling is completed in ticket 09.

## Comments

- Implementation claimed 2026-09-13. Public HTTP/SSE is the behavioral test seam approved by the parent specification; browser coverage uses the controlled demo Kubernetes client.
- Implemented and verified 2026-09-13. Added authorized tenant/type replacement-snapshot SSE, commit-safe subscription setup, bounded reconnect-on-backpressure, session cancellation, UID-safe live list/detail updates, connection indicators, overlap/removal/reconnect/type/session HTTP coverage, and controlled browser live-update coverage. ADR 0023 records the delivery decision. Ticket 09 still owns upstream observer outage recovery.
