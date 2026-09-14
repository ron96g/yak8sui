# 01: Sign in and select an authorized tenant

**What to build:** A working same-origin Go and React/TypeScript/Material UI application in which an administrator signs in through OIDC, selects an authorized tenant, and signs out. Establish the public-interface test harness alongside this first vertical slice.

**Blocked by:** None (can start immediately).

**Status:** ready-for-agent

**Implementation note:** Claimed locally on 2026-09-12 for the approved ticket 01 implementation. Implementation and all required completion checks completed the same day; the tracker status remains the fixed triage role `ready-for-agent`.

## Acceptance criteria

- [x] Backend-managed OIDC login establishes a secure HTTP-only session cookie; authentication failures produce a usable error state.
- [x] Deployment YAML maps identity-provider groups to multiple configured tenants with label selectors. Invalid identity/tenant configuration produces actionable startup errors.
- [x] The browser lists only authorized tenants and can switch between them; tenant identity is explicit in backend requests.
- [x] Backend authorization rejects unauthenticated requests and unauthorized tenant identifiers independently of UI controls.
- [x] Logout and session expiry invalidate access and clear displayed tenant data. Keep the session boundary usable by the later SSE slice.
- [x] A controlled identity provider supports repeatable public HTTP tests for allowed/denied groups, tenant switching, invalid sessions, expiry, and logout.
- [x] A minimal browser smoke test demonstrates sign-in, authorized tenant selection, and logout; document how to run the application and checks.

## Implementation context

Use the approved spec and ADRs 0002, 0006, 0010, and 0014. Begin with the combined application; this slice establishes authenticated tenant context rather than resource inspection.

## Comments

- 2026-09-12: Added the browser smoke test and documented its command. Playwright's native install rejected the detected `ubuntu20.04-x64` platform. An attempted `ubuntu22.04-x64` fallback downloaded but Chromium could not launch because the environment lacks `libgbm.so.1`. This criterion remains unchecked until the browser journey is observed in an environment with supported browser dependencies.
- 2026-09-12: Browser blocker resolved after the user installed `libgbm1`. With `PLAYWRIGHT_HOST_PLATFORM_OVERRIDE=ubuntu22.04-x64`, Chromium installation succeeded and `npm run test:browser` passed the sign-in, authorized tenant selection/switching, and logout smoke test (1 passed). README documents the local workaround. Overall implementation completion still awaits a successful mandatory `golangci-lint run ./...`; lint findings remain unresolved.
- 2026-09-12: Resolved all 35 Go lint findings without changing `.golangci.yml` or application behavior. `golangci-lint run ./...` passed with 0 issues, `go test -race ./...` passed (10 tests across 6 packages), and `PLAYWRIGHT_HOST_PLATFORM_OVERRIDE=ubuntu22.04-x64 npm run test:browser` passed the browser smoke test (1 passed). Ticket 01 now satisfies all implementation completion gates.
