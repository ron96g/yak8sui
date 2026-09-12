# 01: Sign in and select an authorized tenant

**What to build:** A working same-origin Go and React/TypeScript/Material UI application in which an administrator signs in through OIDC, selects an authorized tenant, and signs out. Establish the public-interface test harness alongside this first vertical slice.

**Blocked by:** None (can start immediately).

**Status:** ready-for-agent

## Acceptance criteria

- [ ] Backend-managed OIDC login establishes a secure HTTP-only session cookie; authentication failures produce a usable error state.
- [ ] Deployment YAML maps identity-provider groups to multiple configured tenants with label selectors. Invalid identity/tenant configuration produces actionable startup errors.
- [ ] The browser lists only authorized tenants and can switch between them; tenant identity is explicit in backend requests.
- [ ] Backend authorization rejects unauthenticated requests and unauthorized tenant identifiers independently of UI controls.
- [ ] Logout and session expiry invalidate access and clear displayed tenant data. Keep the session boundary usable by the later SSE slice.
- [ ] A controlled identity provider supports repeatable public HTTP tests for allowed/denied groups, tenant switching, invalid sessions, expiry, and logout.
- [ ] A minimal browser smoke test demonstrates sign-in, authorized tenant selection, and logout; document how to run the application and checks.

## Implementation context

Use the approved spec and ADRs 0002, 0006, 0010, and 0014. Begin with the combined application; this slice establishes authenticated tenant context rather than resource inspection.
