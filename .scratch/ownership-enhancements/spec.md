# YAK8SUI: navigable and extensible ownership inspection

**Status:** ready-for-agent

## Problem Statement

Ownership is central to diagnosing Kubernetes reconciliation, but the current visual hierarchy becomes difficult to scan as more branches are expanded. Inspecting a related node also recenters the resource detail, discards the user's current ownership context, and requires navigation back to continue comparing resources. Finally, ownership edges can only be resolved from `metadata.ownerReferences`, while real controllers often express logical ownership through matching labels or structured references in `spec` or `status`.

## Solution

Make the loaded ownership hierarchy available as either the existing directional graph or a compact table without changing what has been expanded. Let users select any visible node and inspect its live YAML beside the hierarchy without changing the center resource, with a separate explicit action for recentering.

Extend restart-required relationship configuration with composite label-join and structured field-reference resolvers. Preserve tenant isolation, observation freshness, explicit type scope, Kubernetes scope validity, and transparent completeness across every resolver.

## User Stories

1. As an administrator, I want to switch between graph and table presentations, so that I can choose the clearest representation for the hierarchy's size and shape.
2. As an administrator, I want both presentations to show the same loaded hierarchy, so that changing presentation does not discard my exploration.
3. As an administrator, I want presentation changes to preserve expanded and collapsed branches, so that I can compare views without repeating work.
4. As an administrator, I want table mode to remain lazy, so that merely changing presentation does not query the entire configured hierarchy.
5. As an administrator, I want table rows to show ancestry and direction, so that ownership remains unambiguous outside the graph layout.
6. As an administrator, I want resource name, kind, namespace, health, controller ownership, and depth visible in the table, so that I can scan useful evidence quickly.
7. As an administrator, I want to expand and collapse branches from either presentation, so that both modes support the same navigation workflow.
8. As an administrator, I want incomplete, unavailable, configuration-limited, ambiguous, and cyclic branches visible in either presentation, so that missing rows are not mistaken for authoritative absence.
9. As an administrator, I want graph and table modes to remain usable on narrow screens, so that ownership inspection does not require a desktop viewport.
10. As an administrator, I want hierarchy subscriptions shared across presentations, so that switching views does not duplicate live streams or briefly show inconsistent data.
11. As an administrator, I want to select a graph node and immediately see its YAML, so that I can inspect related resources without losing context.
12. As an administrator, I want to select a table row and immediately see the same YAML preview, so that inspection behavior is consistent between presentations.
13. As an administrator, I want selecting another node to replace the preview in place, so that I can rapidly compare related resources.
14. As an administrator, I want the hierarchy center and expansion state preserved while previewing, so that inspection does not restart traversal.
15. As an administrator, I want an explicit action to explore from the previewed resource, so that recentering is intentional rather than a side effect of inspection.
16. As an administrator, I want previewed YAML to update live, so that reconciliation changes are visible while I inspect a related resource.
17. As an administrator, I want preview loading, reconnecting, observation loss, and access revocation shown distinctly, so that stale YAML is never presented as current.
18. As a tenant user, I want every YAML preview independently authorized, so that ownership cannot expose another tenant's resource contents.
19. As a tenant user, I want tenant, center, or node changes to cancel obsolete preview work, so that a late response cannot appear in the wrong context.
20. As an operator, I want one resolver selected per configured parent/child type pair, so that ownership semantics are explicit and predictable.
21. As an operator, I want existing owner-reference rules to continue working unchanged, so that the enhancement does not alter established ownership.
22. As an operator, I want a label-join resolver with separately configured parent and child label keys, so that differently named labels can express the same logical relationship.
23. As an operator, I want label joins to support multiple key pairs combined with AND, so that broad label values do not create accidental edges.
24. As an operator, I want every configured label on both endpoints to be present, so that missing values cannot compare equal accidentally.
25. As an operator, I want label ownership to follow a replacement parent with the same joined values, so that logical ownership survives recreation by design.
26. As an operator, I want a field-reference resolver for structured references in child spec or status, so that controller-specific references can define ownership.
27. As an operator, I want field references to use familiar ObjectReference fields, so that API version, kind, name, UID, and namespace semantics are explicit.
28. As an operator, I want field references to resolve exactly one object rather than arrays, so that this increment remains simple and deterministic.
29. As an operator, I want field-reference identity checked by UID, so that a recreated parent is not silently substituted.
30. As an administrator, I want custom ownership to work upward and downward, so that resolver choice does not reduce hierarchy navigation.
31. As a tenant user, I want each custom-resolved parent and child independently tenant-authorized, so that custom matching never grants access.
32. As an administrator, I want unavailable custom-rule endpoint types reported as incomplete, so that observation failure is not represented as an empty hierarchy.
33. As an administrator, I want an ambiguous custom match to return no arbitrary parent and mark the branch incomplete, so that the UI does not invent ownership.
34. As an operator, I want invalid resolver configuration rejected at startup with a precise path, so that unsafe or contradictory rules cannot run.
35. As an operator, I want relationship changes to refresh open graph, table, and YAML views, so that all presentations reflect committed projection state.

## Implementation Decisions

### Shared ownership hierarchy

- Treat the graph and table as presentations over one hierarchy-state module. The module owns loaded neighborhood snapshots, directional expansion, path-based cycle protection, completeness, problems, and one live subscription per expanded node.
- Keep the current immediate-neighborhood HTTP/SSE interface and replacement-snapshot semantics. Presentation changes do not open duplicate streams and do not reset loaded state.
- The table displays the loaded hierarchy, not the entire possible hierarchy. Expanding a table row loads the same directional neighborhood that graph expansion would load.
- Preserve path occurrences rather than globally deduplicating resources. Owner-reference rules still support multiple parents and graph convergence even though custom rules enforce one parent per child.
- Include direction, depth, immediate parent, resource identity, namespace, kind, health, controller marker, branch state, and actions in table rows. Use type and UID as identity; never use name alone.
- Keep the selected center visually and accessibly distinct in both modes. Graph mode retains owners above, children below, horizontal sibling levels, and existing directional connectors.
- Reset hierarchy state when tenant or center identity changes. A revision invalidates loaded snapshots without conflating presentation mode with data identity.

### In-place YAML preview

- Separate the center resource from the preview selection. Selecting a node changes only the ownership preview; recentering requires a distinct “Explore from this resource” action.
- Add a dedicated UID-scoped detail replacement stream for preview freshness. It returns the same complete detail/YAML representation as an authorized detail read and follows the existing commit-safe replacement-snapshot and session-cancellation rules.
- Reauthorize the tenant and resource on stream establishment and every replacement query. Observation loss and access revocation clear or visibly invalidate the preview rather than retaining apparently current YAML.
- Guard preview state by tenant, center, selected node identity, and request/stream generation so obsolete fetches and events cannot update the current view.
- Reuse the existing read-only YAML viewer. Place preview beside the hierarchy on desktop and below it on narrow screens with explicit bounded height and no document-level horizontal overflow.
- Keep node health and YAML current from committed projection state. Closing ownership, changing tenant, recentering, or selecting another node closes the obsolete preview stream.

### Resolver configuration and dispatch

- Each directional parent/child group-kind pair has exactly one rule and one resolver: `ownerReferences`, `labelJoin`, or `fieldReference`. Omitted resolver continues to mean `ownerReferences`.
- Use a resolver registry/composite behind the existing ownership resolution interface. Dispatch each rule to its adapter and combine results into the unchanged immediate-neighborhood model.
- Keep resolver implementations behind a narrow authorized object-reader interface in the resource module. Resolver adapters receive only tenant-authorized, currently observed candidates and cannot bypass membership or freshness checks.
- Both endpoint types must remain selected in observation configuration. Relationship configuration never broadens discovery, permissions, or tenant access.
- Apply existing scope validity to every resolver: a cluster-scoped child cannot have a namespaced parent, and namespaced parents and children must share a namespace. A cluster-scoped parent may own a namespaced child.
- Keep conservative type-level invalidation initially. A commit to a configured endpoint type reruns affected loaded neighborhoods; do not introduce a separate relationship projection or cache in this effort.

### Composite label joins

- A label-join rule contains a non-empty list of mappings from a parent label key to a child label key. Keys may differ between endpoints.
- An edge exists only when every configured key is present on its endpoint and all mapped values are equal. Empty or missing labels do not match by omission.
- Label joins express logical identity. A newly observed parent with matching labels may replace a deleted parent, and the resulting node/edge uses that parent's current metadata UID.
- Custom resolvers permit at most one matching parent per child. If multiple authorized current parents match, return no edge for that child and mark the affected direction incomplete with a non-sensitive ambiguity diagnostic.
- Multiple children may match one parent. Upward and downward traversal apply the same matching rule and deterministic resource ordering.

### Structured field references

- A field-reference rule contains one validated path rooted under the child's `spec` or `status`. The selected value must be one object, not an array.
- The object follows a Kubernetes ObjectReference subset: `apiVersion`, `kind`, `name`, `uid`, and `namespace` when needed for namespaced resolution. Empty or malformed required fields do not establish an edge.
- The configured relationship supplies the permitted parent and child types. The reference's API group, served version, and kind must agree with the selected parent type; its name, UID, and namespace must identify the same authorized current parent.
- Field-reference ownership is UID-stable. Parent deletion/recreation under the same name does not rebind until the child's reference contains the replacement UID.
- A malformed or mismatched reference produces no edge and a safe incomplete diagnostic rather than a server failure or disclosure of candidate contents.
- Resolver-specific configuration is mutually exclusive. Startup validation rejects missing options, unknown fields, paths outside spec/status, empty/duplicate label mappings, options belonging to another resolver, and duplicate parent/child type pairs.

### Completeness and diagnostics

- Extend directional completeness so resolver ambiguity and malformed custom references can be distinguished from unavailable observation and configuration-limited traversal.
- Diagnostics identify the configured relationship and general reason but do not reveal names, UIDs, labels, references, or counts for inaccessible candidates.
- A healthy rule with no match is authoritatively empty. A rule that cannot resolve safely because it is unavailable, ambiguous, or malformed is incomplete.

## Testing Decisions

- Test externally observable behavior primarily through the running application's authenticated HTTP/SSE interface with the controlled Kubernetes API and identity provider.
- Extend browser acceptance coverage for Graph/Table switching, shared branch expansion, lazy loading, row ancestry, cycle and completeness display, node selection, live YAML replacement, explicit recentering, tenant changes, revocation, and mobile layout.
- Assert that presentation toggles do not duplicate ownership subscriptions and preserve loaded branch state.
- Test the preview stream's snapshot-to-update handoff, reconnect, session expiry, observation loss, deletion/recreation, tenant membership removal, and stale-response suppression.
- Exercise owner-reference, label-join, and field-reference rules together across different type pairs to verify per-rule dispatch without changing the neighborhood interface.
- Cover composite label equality, differing endpoint keys, missing labels, multiple children, dynamic parent replacement, zero matches, and ambiguous parents.
- Cover valid spec and status paths, core and non-core API versions, namespace requirements, name/UID/type mismatches, malformed values, unsupported arrays, recreation with stale UID, and unavailable endpoint types.
- Verify all candidate resources are independently tenant-scoped and that diagnostics do not expose inaccessible objects.
- Preserve existing owner-reference tests, including multiple parents, controller markers, UID identity, scope checks, cycle handling, and live updates.
- Run Go tests, race tests where used by the repository, mandatory `golangci-lint run ./...`, frontend formatting/typecheck/build, React Doctor, and Playwright browser/demo checks for implementation tickets.

## Out of Scope

- Automatic discovery or inference of ownership rules.
- Arbitrary expressions, templates, scripts, regex, Kubernetes LabelSelector syntax, or OR semantics in label joins.
- Field-reference arrays, references stored outside child spec/status, or parent-to-child field references.
- More than one custom-resolved parent per child, precedence between competing custom rules, or multiple rules for the same parent/child type pair.
- Changing owner-reference rules to enforce the custom-resolver single-parent invariant.
- Persisting a separate relationship graph, relationship history, or relationship-specific database indexes before measured need.
- Editing resource YAML or ownership configuration from the browser.
- Automatically loading the entire hierarchy merely by selecting table mode.

## Further Notes

- ADRs 0033–0035 record the shared-presentation, live-preview, and custom-resolver decisions.
- The graph remains the default presentation. Table mode is an alternate navigation aid, not a replacement for directional graph semantics.
- Tickets 01 and 03 can begin independently. Ticket 02 follows the hierarchy-state extraction in ticket 01, and ticket 04 follows resolver dispatch and label-join behavior in ticket 03.
