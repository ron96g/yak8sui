# 04: Keep tenant views live and authorized

**What to build:** Tenant users see resource changes automatically through authorized SSE snapshots and updates, including removal when membership changes and clean recovery after browser reconnects.

**Blocked by:** 03 — Discover the configured resource scope.

**Status:** ready-for-agent

## Acceptance criteria

- [ ] Native EventSource or a suitable existing client/protocol implementation consumes an authorized initial snapshot followed by resource updates.
- [ ] The snapshot-to-update handoff does not lose changes arriving during connection setup; updates correspond to committed projection state.
- [ ] Browser disconnect/reconnect displays connection/resynchronization state, reauthorizes the tenant, and replaces the view with a fresh snapshot rather than replaying history.
- [ ] Label changes remove resources from tenants they no longer match, including an open detail; shared-selector resources are visible in each authorized matching tenant.
- [ ] Tenant switching clears the previous tenant's view and subscriptions; updates from a prior tenant cannot populate the new view.
- [ ] Logout and session expiry close streams and clear displayed tenant data. A stream cannot outlive its authorized session.
- [ ] Selection follows UID; deleting and recreating a same-named object cannot silently substitute it as the old selected object.
- [ ] HTTP/SSE tests exercise concurrent snapshot changes, multiple tenants, overlap, membership removal, reconnect, and expiry; a browser check demonstrates automatic updates.

## Implementation context

Use ADRs 0002, 0003, and 0010. Keep live invalidation usable by subsequent paginated-query and hierarchy views. Database observation failure handling is completed in ticket 09.
