# YAK8SUI: tenant-isolated resource inspection

**Status:** ready-for-agent

## Problem Statement

Administrators need to understand the current state of selected Kubernetes Custom Resources, determine whether reconciliation is healthy, and trace ownership to relevant parents and descendants. Raw resource YAML alone makes this work cumbersome. A shared cluster also requires tenant isolation: users must only see resources selected for tenants they are authorized to access, across every inspection surface.

The repository currently contains design diagrams and accepted ADRs, but no application implementation or existing test suite. The diagrams are historical design input; the accepted read-only scope supersedes their mutation controls.

## Solution

Build a same-origin, read-only admin application that discovers explicitly configured resource types, observes their current state, and presents tenant-scoped lists and detail views. Display complete resource YAML, readable conditions, Ready-based health, and observation freshness. Deliver live updates and searchable, paginated lists without weakening tenant boundaries.

Complete the workflow with an expandable ownership view and explicitly requested Kubernetes Events. The administrator diagnoses reconciliation using this evidence; automated diagnosis is a future capability.

## User Stories

1. As an administrator, I want to sign in through OIDC, so that my existing identity determines access.
2. As an administrator, I want to select from my authorized tenants, so that each inspection session has an explicit tenant context.
3. As an operator, I want identity-provider groups mapped to tenants, so that access can be configured centrally.
4. As a tenant user, I want isolation enforced on every backend read and stream, so that another tenant's contents cannot be exposed by changing a request.
5. As an operator, I want resource labels to determine tenant membership, so that existing cluster labeling controls visibility.
6. As an operator, I want overlapping tenant selectors to permit shared resources, so that intentional sharing is supported.
7. As a tenant user, I want resources removed when their labels stop matching, so that my live view does not retain unauthorized membership.
8. As an operator, I want to select CRD API groups and resource-type overrides, so that observation covers an intentional subset of the cluster.
9. As an operator, I want per-type selectors to further restrict observation, so that type-specific rules cannot relax tenant boundaries.
10. As an operator, I want invalid configuration rejected clearly, so that deployment mistakes are actionable.
11. As an administrator, I want both namespaced and cluster-scoped CRs, so that scope does not hide relevant resource types.
12. As an administrator, I want one selected served version per resource type, so that resources do not appear duplicated across API versions.
13. As an administrator, I want GroupVersion/Kind navigation, so that I can find configured resource types.
14. As an administrator, I want to inspect complete resource YAML in a read-only view, so that arbitrary spec and status fields remain accessible.
15. As an administrator, I want conditions with reasons and messages, so that controller-reported evidence is readable.
16. As an administrator, I want health derived from Ready, so that healthy, unhealthy, and unknown resources can be distinguished.
17. As an administrator, I want outdated observed generations marked stale, so that an old condition is not mistaken for confirmation of the latest desired state.
18. As an administrator, I want resource state to update automatically, so that I can follow reconciliation without repeatedly refreshing.
19. As an administrator, I want reconnects to replace my view with a fresh authorized snapshot, so that missed updates do not leave inconsistent state.
20. As an administrator, I want connection and synchronization indicators, so that I know whether displayed information is current.
21. As an administrator, I want fuzzy matching for names and namespaces, so that partial ordered-character searches find relevant resources.
22. As an administrator, I want regex mode with visible validation errors, so that more precise matching is available when needed.
23. As an administrator, I want namespace filters ignored for cluster-scoped resources, so that an irrelevant filter cannot hide them.
24. As an administrator, I want JSONPath existence and typed equality filters, so that arbitrary resource fields can narrow a list.
25. As an administrator, I want array wildcard queries to match any selected value, so that lists such as conditions can be searched.
26. As an administrator, I want all filters applied before pagination and counting, so that matching resources are found beyond the currently loaded page.
27. As an administrator, I want stable ordering and selection by UID, so that updates and resource recreation do not silently change what I am inspecting.
28. As an operator, I want best-effort native SQL execution and indexes, so that queries avoid unnecessary whole-cluster work.
29. As an operator, I want SQLite and PostgreSQL deployment choices with equivalent behavior, so that storage choice does not change inspection semantics.
30. As an operator, I want the latest observed state retained across restarts, so that the projection can be recovered and synchronized without requiring history storage.
31. As a tenant user, I want unsynchronized cached contents withheld on new reads, so that old tenant labels cannot grant access after observation loss.
32. As an administrator, I want healthy types to remain usable when another type is unavailable, so that partial failures do not block all work.
33. As an administrator, I want unavailable and incomplete results distinguished from authoritative empty results, so that missing evidence is apparent.
34. As an operator, I want permitted ownership relationships described in YAML, so that related resource observation stays explicit.
35. As an administrator, I want to traverse both parents and children, including configured built-in resource types, so that I can inspect the authorized ownership chain.
36. As an administrator, I want multiple parents, controller-owner markers, cycle protection, and lazy branch expansion, so that ownership remains navigable beyond a simple tree.
37. As a tenant user, I want every related resource independently checked against tenant labels, so that ownership never implicitly grants access.
38. As an operator, I want an extensible relationship resolver boundary, so that future YAML rules can select label or field-reference resolvers per relationship.
39. As an administrator, I want Events fetched only when I open their tab or refresh it, so that viewing resource state does not continuously fetch Events.
40. As an administrator, I want Events correlated by resource UID, so that a recreated resource does not inherit the previous object's Events.
41. As an administrator, I want logout and session expiry to close streams and clear tenant data, so that the browser respects session boundaries.
42. As an operator, I want a deployable combined ingestion/API application and reproducible workload checks, so that the initial deployment is practical and its capacity claims are measured.

## Implementation Decisions

### Stack and boundaries

- Use Go and Kubernetes client-go discovery/informers for the backend; React, TypeScript, Material UI, and a read-only Monaco YAML viewer for the frontend.
- Initially run ingestion and API serving in one process. Keep configuration/discovery, identity/tenant authorization, observation/projection, resource queries, live delivery, relationship resolution, and requested Events as internally separated responsibilities.
- Prefer existing protocol implementations and libraries. Specific dependencies, schemas, endpoint names, and detailed SLAs are implementation decisions to document as needed.
- The public application HTTP/SSE interface is the primary behavioral test boundary. Internal separation does not imply separate deployed services.

### Identity, selection, and observation

- Serve the frontend and API from the same origin. The backend handles OIDC login and secure HTTP-only session cookies; deployment configuration maps identity-provider groups to tenants.
- Recheck tenant authorization on every request and stream establishment. Bound streams by session lifetime; logout and expiry close streams and clear displayed tenant contents.
- Support multiple tenants in the first increment. Each resource independently satisfies its tenant selector and applicable type restrictions. Shared label membership is intentional; membership changes update lists, details, and streams.
- Group/type selections, type restrictions, and ownership rules are shared across tenants. Tenant label selectors differ by tenant.
- Observe selected CRD groups with explicit type inclusions/exclusions and optional per-type selectors. Exclusions win. Include additional configured relationship resource types as needed; do not infer permission to observe the entire cluster.
- Support namespaced and cluster-scoped resources. Configuration responses expose type scope and observation availability to the UI.
- Choose one served version per group/kind, defaulting to the API's preferred served version with an override. Resource identity and ownership resolution preserve UID identity across deletion and recreation.
- Validate restart-required YAML configuration at startup. Malformed or contradictory configuration prevents startup; valid rules referencing unavailable dependencies instead produce affected-type unavailability.

### Projection, availability, and delivery

- Persist a rebuildable latest-state projection in SQLite initially, then provide equivalent PostgreSQL behavior. Retain current observed resources rather than resource-version history.
- Preserve last observed state across restarts but withhold contents on new reads until the relevant observation scope is synchronized. After observation loss, already-open views become visibly stale and stop receiving updates; synchronized types remain usable.
- Distinguish observation freshness from condition-generation freshness. Expose disconnected, resynchronizing, and unavailable states rather than presenting unavailable resources as an authoritative empty list.
- Use SSE: initial snapshot followed by updates; reconnect reauthorizes and replaces the view with a fresh snapshot. Ensure the snapshot-to-update transition cannot lose a committed change. Historical replay is unnecessary.
- Preserve selection by UID. Live list changes refresh the matching page and count, including resources newly entering or leaving the result set.

### Resource presentation and health

- Display complete resource YAML and readable conditions, including reason and message. The application is read-only.
- Evaluate the condition with type Ready: True is Healthy, False is Unhealthy, Unknown or absent is Unknown. Apply this rule to built-in related resources too.
- A condition whose observedGeneration trails metadata.generation is Stale; preserve its reported state and evidence. Without observedGeneration, use the reported condition without asserting generation freshness.

### SQL-first filtering and pagination

- Search all authorized observed resources of the selected type. AND Name, Namespace, and JSONPath filters before pagination and counting.
- Name and Namespace support case-insensitive ordered-character fuzzy matching. Regex mode uses Go regular-expression semantics, case-sensitive by default with inline case-insensitive flags available. Return clear validation errors for invalid or unsupported patterns.
- Cluster-scoped resources ignore namespace filtering; the UI disables the namespace input using configuration scope information.
- Use an existing JSONPath parser/validator where suitable, accepting field access, array indexes, and array wildcards. No comparison means path existence; an explicit typed JSON comparison means equality with any selected value. Present null exists; missing does not. Boolean and string values are distinct. Reject predicates, scripts, recursive descent, and other unsupported syntax.
- Library-backed parsing does not imply application-side query execution. Translate supported operations into native database predicates wherever equivalent semantics are available. Use parameterized queries and database-specific adapters without changing the public filter contract.
- Restrict candidates by indexed tenant membership and resource type first. Push further predicates into SQL when faithful. Where necessary, evaluate remaining predicates over scoped candidates before paginating matching results; native database regex must not silently replace Go semantics.
- Stable ordering is namespace/name, with 100-row pages initially. The count and returned page represent the same filtered dataset. Internal candidate batching must not become pagination-before-filtering.
- Arbitrary queries are not guaranteed to be index-backed. Inspect plans for indexed scoping and add targeted JSON-path indexes as workloads justify them; avoid unnecessary whole-cluster scans.

### Ownership and Events

- YAML identifies permitted parent/child type relationships by API group and kind. Actual initial edges require ownerReferences resolved by UID, not naming similarity or the type rule alone.
- Keep resolution extensible so future YAML can choose a specific resolver per relationship; implement owner-reference resolution now.
- Present the selected resource with expandable parents above and children below. Include configured built-in types, support multiple parents, identify controller-owner edges, and guard against cycles.
- Load branches on demand while retaining navigation through the full authorized configured chain. Stop at inaccessible resources without exposing their fetched contents. Mark limited or unavailable branches incomplete without crossing tenant boundaries.
- Fetch Kubernetes Events only on Events-tab activation or manual refresh, scoped to the selected resource UID and authorized tenant membership. Resource detail opening alone causes no Events fetch; do not ingest or continuously subscribe to Events.

## Testing Decisions

- Test externally observable behavior, not internal call sequences or module wiring. The repository has no existing test implementation to reuse.
- Use the running application's public HTTP/SSE boundary with a controlled Kubernetes API and test identity provider. Exercise authentication, membership changes, projection, query results, delivery, availability, ownership, and Events through that boundary.
- Use representative Kubernetes list/watch/discovery behavior, including changes during snapshot establishment, deletion/recreation, permission denial, observation loss, resynchronization, and independently failing resource types.
- Run persistence-sensitive behavioral cases against real SQLite and PostgreSQL. Compare result sets and counts for fuzzy, Go-regex, JSONPath null/missing, typed equality, wildcard matches, and combinations across multiple pages.
- Verify matches beyond the first candidate batch and exclusion of other tenants; no optimization may introduce false negatives. Inspect query plans for indexed tenant/type scoping and representative SQL pushdown as a targeted execution check, without coupling behavioral tests to exact SQL text.
- Add a small browser suite covering login/logout, tenant switching, filters, pagination, live state and health, ownership expansion, and explicit Events activation. Observe that no Events request occurs before activation.
- Test session expiry, arbitrary tenant identifiers, label removal, overlapping selectors, related resources lacking tenant labels, and stale membership after restart at the public interface.
- Check the provisional workload of 10,000 observed resources, 20 tenants, and 20 simultaneous viewers. Measure ordinary browser visibility within two seconds of backend observation under a documented workload; record environment, update rate, failures, and limitations. These are acceptance targets, not a formal SLA or an assertion about every possible query.

## Out of Scope

- Resource creation, YAML edits, deletion, and controller trigger actions.
- Agent-driven or controller-specific automated diagnosis and kind-specific health adapters.
- Multi-cluster management, separately deployed ingestion/API replicas, and HA coordination in the first deployment.
- Resource-version history, Event history persistence, continuous Event subscriptions, and offline serving of unsynchronized cached contents.
- Automatic discovery of ownership rules, implemented label/spec/status relationship resolvers, and ownership-based access grants.
- Live configuration reload and tenant-specific group/type/relationship overrides.
- General-purpose JSON query expressions beyond the agreed subset, universally indexed arbitrary queries, and detailed production SLAs.

## Further Notes

- Accepted ADRs 0001–0013 govern this spec; ADR 0014 records the approved delivery/test plan and SQL-first clarification.
- Tickets 01–09 form the initial inspection increment. Tickets 10–11 add ownership and requested Events; ticket 12 packages and verifies the complete workflow. Dependencies, not ticket numbers alone, determine which work can start.
- Every ticket is a demoable vertical slice, including relevant persistence, API, UI, and verification. There is no existing application requiring a prefactoring phase.
- The 12-ticket breakdown was approved after clarification that JSONPath parsing and SQL query execution are separate concerns. Local ticket readiness does not bypass blockers.
