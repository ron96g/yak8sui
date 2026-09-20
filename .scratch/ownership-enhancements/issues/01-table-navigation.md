# 01: Navigate the loaded ownership hierarchy as a table

**What to build:** An administrator can switch the current authorized ownership hierarchy between graph and table presentations while retaining the same loaded branches, live updates, completeness, and navigation state.

**Blocked by:** None (can start immediately).

**Status:** in-progress

## Acceptance criteria

- [x] Add an accessible Graph/Table control; graph remains the default presentation.
- [x] Extract loaded neighborhoods, directional expansion, cycle protection, completeness, errors, and subscription ownership into shared hierarchy state consumed by both presentations.
- [x] Switching presentation preserves expanded/collapsed branches and loaded snapshots without opening duplicate SSE subscriptions or automatically expanding the entire hierarchy.
- [x] Table rows expose direction/depth, immediate parent, resource name, kind, namespace, health, controller ownership, branch state, and node actions with type/UID identity.
- [x] Expansion and collapse from either presentation affect the same hierarchy state, including Expand entire hierarchy and Collapse branches.
- [x] Preserve one occurrence per loaded ownership path so existing multiple-parent owner-reference graphs, convergent paths, and path-based cycle stops remain representable.
- [x] Show unavailable, configuration-limited, incomplete, and cyclic branches without representing them as authoritative empty results.
- [x] Keep the graph's directional layout and narrow-screen sibling scrolling unchanged; make the table usable at 320px and avoid document-level horizontal overflow.
- [ ] HTTP/SSE and browser tests demonstrate lazy loading, live edge changes, mode switching without duplicate streams, preserved expansion, multiple paths, cycle handling, tenant switching, and responsive navigation.
- [x] Record the shared hierarchy-state interface and any changed accessibility language in supporting documentation.

## Implementation context

Use ADRs 0030, 0031, and 0033. This ticket changes ownership state placement and presentation, not resolver behavior or resource authorization.

## Progress comments

- Added one shared hierarchy-state hook for directional path occurrences, snapshots, errors, expansion, cycle stops, and node-keyed stream ownership.
- Added the responsive loaded-hierarchy table and preserved the graph as the default presentation.
- Extended Playwright coverage for preserved expansion, relationship-stream request counts across presentation switches, and 320px document overflow.
- Existing resolver coverage protects multiple-owner behavior, while dedicated browser fixtures for convergent paths and actual cycles remain to complete the full acceptance criterion.
- Current implementation is verified with frontend formatting/typecheck/build, the complete browser suite, React Doctor, `go test ./...`, and `golangci-lint run ./...`.
