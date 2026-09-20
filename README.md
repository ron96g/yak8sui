# Yet Another Kubernetes UI (github.com/ron96g/yak8sui)

UI that is used as an admin-tool to view the state of Custom-Resources (mostly).

## Resource inspection (tickets 01–03)

### Kubernetes deployment

See [deploy/README.md](deploy/README.md) for the production container build,
Kustomize base/example overlay, OIDC Secret configuration, persistent SQLite,
resource-scoped RBAC, and GitHub Actions publishing to GHCR.

The first vertical slice is a combined same-origin Go server and React/TypeScript/Material UI application. It performs backend-managed OIDC Authorization Code login with PKCE, stores opaque sessions in secure HTTP-only cookies, maps verified groups to configured tenants, authorizes every tenant request, and supports tenant switching and logout.

### Explore the local demo

The local demo requires Go 1.24+ and Node.js 22+. It uses the real combined
application, OIDC authorization-code/session flow, tenant authorization, resource
HTTP APIs, SQLite projection, and React UI. The identity provider and Kubernetes
dynamic client are controlled in-process simulations seeded with demo-only data;
**this is not a real Kubernetes cluster deployment and is not for production**.

From the repository root, build the frontend and start the foreground demo:

```sh
npm --prefix frontend ci
npm --prefix frontend run build
go run ./cmd/localdemo
```

Open **http://127.0.0.1:19080** and select **Sign in with OIDC**. The controlled
provider signs in `Demo Operator` without external credentials. Select Platform
to inspect the namespaced `platform-api` Widget and cluster-scoped
`platform-guardrails` ClusterPolicy, then Payments to inspect their tenant
counterparts. The controlled discovery fixture demonstrates two API groups and
a `v1beta1` override. Complete resources are shown as read-only YAML. An
`internal-only` Widget is outside both tenant selectors and must not appear.
Name and Namespace search updates automatically after a 300ms typing pause,
including mode changes; new filters return to page one. No Apply action is needed.
JSONPath edits, Exists/Equals changes, and typed JSON values use the same automatic
search. Examples include `$.status.conditions` (present, including null),
`$.spec.replicas` Equals `3`, and `$.status.conditions[*].type` Equals `"Ready"`.
Equals accepts one JSON string, number, boolean, or null, so `true` and `"true"`
are different and `null` differs from a missing path. Use quoted access such as
`$['metadata']['labels']['app.kubernetes.io/name']` for punctuation in field names.
Only fields, non-negative array indexes, and array wildcards are supported;
predicates, scripts, recursive descent, slices, unions, and object wildcards are
rejected.

The demo also seeds 105 `platform-search-NNN` Widgets in `search-lab`: fuzzy
Name `ps104` plus Namespace `sl` returns one result, while Regex
`(?i)^platform-search` demonstrates two accurately counted pages. Name and
Namespace filters are combined with AND; namespace controls are disabled for
ClusterPolicy because it is cluster scoped.

With fuzzy `ps104` applied, this demo-only route moves that resource out of or
back into the filtered live page:

```sh
curl -X POST http://127.0.0.1:19080/demo/simulate/search-update
```

To demonstrate live delivery, open `platform-api` and run the following demo-only
simulation in another terminal. Repeated calls cycle its Ready evidence through
stale, unhealthy, and healthy states while toggling the seeded replica count;
the table, readable conditions, and open YAML update without a browser refresh.
This route exists only in `localdemo` and the browser fixture, not in the
production server:

```sh
curl -X POST http://127.0.0.1:19080/demo/simulate/widget-update
```

To interrupt Widget observation, change `platform-api` tenant membership while
the observer is unavailable, and then let the bounded runtime retry perform a
fresh authoritative relist, run:

```sh
curl -X POST http://127.0.0.1:19080/demo/simulate/observation-loss
```

The open Widget view remains visible but is explicitly marked unavailable and
does not receive resource snapshots until recovery. The next successful relist
removes `platform-api` from Platform (or restores it on the next invocation)
before observation becomes current. ClusterPolicy remains usable throughout.
This loopback-only control uses the production observer cancellation, freshness,
retry, and relist path, but the upstream Kubernetes API and failure trigger are
controlled in-process simulations rather than a real network or API server.

Open `platform-api`, then choose **Ownership**. Read downward: each connector
states which resource owns the next. The selected resource has a highlighted border.
Use the labelled **Graph/Table** control to switch between the directional graph
and a compact table of the same loaded hierarchy. The table reports each row's
direction, depth, predecessor relationship, identity, health, controller marker,
and branch state. Switching views preserves loaded and collapsed branches and
does not load additional neighborhoods; on narrow screens the table scrolls
within the ownership view.
The demo has five resources across four levels:

```text
ClusterPolicy platform-guardrails
  owns ConfigMap platform-settings
    owns Widget platform-api
      owns ConfigMap platform-runtime
      owns ConfigMap platform-routes
```

Click **Show owners** on `platform-settings` to reveal `platform-guardrails`.
Use **Expand entire hierarchy** to load all accessible ancestors and descendants
automatically, or **Collapse branches** to return to the immediate neighborhood.
Inspect `platform-settings`, then **Show owned resources** on `platform-api` to
expand the descendant branch. Inspect opens a resource's Conditions/YAML and lets
you explore from that resource. Every edge is verified by owner-reference UID;
configured type pairs alone create no edges. This demo-only control removes or
restores the `platform-settings` → `platform-api` link live:

```sh
curl -X POST http://127.0.0.1:19080/demo/simulate/ownership-update
```

Ownership is not an access grant: every displayed node must independently match
the active tenant and type selectors. Unavailable configured types make a branch
incomplete; unconfigured directions are labeled configuration-limited.

To see a resource enter and leave a live JSON-filtered page, filter JSONPath
`$.spec.replicas` with Equals `3`, open `platform-api`, and call the route above.
Each call toggles replicas between 3 and 4; the selected detail remains open when
the row leaves the filter because the resource is still authorized.

SQLite executes existence and scalar equality natively through parameterized,
correlated `json_each` traversal after indexed tenant/type scoping. Correlated
existence avoids duplicate rows for multiple wildcard matches. Non-integer number
equality is the only JSON fallback: SQLite can erase source numeric precision, so
those values are compared as exact JSON numbers over the already scoped candidates
before count and pagination. Arbitrary JSON paths are not automatically indexed.

Stop with `Ctrl-C`. The scoped projection persists at
`.local/demo/resources.db`; the simulated Kubernetes snapshot is authoritative,
so every start reconciles it back to the seeded resources. To reset only demo
state while stopped:

```sh
rm -rf .local/demo
```

The demo binds only loopback addresses and uses ports 19080 (application) and
19081 (controlled OIDC), avoiding Playwright's 18080/18081 fixture ports. If a
port is occupied, startup fails with its address; choose a matched set explicitly:

```sh
go run ./cmd/localdemo \
  -address 127.0.0.1:29080 -public-url http://127.0.0.1:29080 \
  -idp-address 127.0.0.1:29081 -idp-url http://127.0.0.1:29081
```

To launch this exact demo under Playwright and verify the complete browser flow,
run `npm --prefix frontend run test:demo` after building. Playwright starts and
stops the demo itself for this check.

For a real cluster and identity provider, use the production `cmd/yak8sui`
workflow below. Its `observation.kubeconfig` supports a local kubeconfig; when
omitted it uses in-cluster service-account credentials.

### Run

Requirements: Go 1.24+, Node.js 22+, and an OIDC provider.

```sh
cd frontend && npm install && npm run build && cd ..
cp config/example.yaml config/local.yaml
# Edit issuer/client/redirect settings and tenant mappings. Use HTTPS in deployment.
go run ./cmd/yak8sui -config config/local.yaml
```

Configuration is restart-required. `cookieSecure` defaults to true for an HTTPS `publicURL`; local plain-HTTP development may explicitly set `cookieSecure: false`. Client secrets should be injected into the local/deployment file from a secret manager and must not be committed.

Configure API groups under `observation.groups` and a writable SQLite
`databasePath`. Each group can include an explicit resource set (or all
discovered resources when omitted), exclude resources, and supply per-resource
served-version and label-selector overrides. Exclusions win, and type selectors
further restrict rather than replace tenant selection. The process uses the
configured kubeconfig when present, otherwise standard in-cluster credentials.
It discovers one served version and Kubernetes scope per selected type, runs
independent client-go informers, and serves tenant-scoped reads only after that
type's initial synchronization. Temporarily unavailable explicit types remain
visible as unavailable without blocking healthy types.

Declare `observation.relationships` as parent/child API-group and kind pairs.
Both endpoint types must be selected in `observation.groups`; use an empty group
for core Kubernetes types. `ownerReferences` is the default and only supported
resolver. Edges require exact owner API group/version, kind, and UID matches;
the owner-reference name is descriptive rather than identity.

### Verify

```sh
go test ./...
golangci-lint run ./...
cd frontend && npm ci && npm run typecheck && npm run build
cd frontend && npx playwright install --with-deps chromium && npm run test:browser
```

The Go acceptance tests launch a controlled OIDC provider and exercise the public HTTP boundary without manual identity-provider setup. The Playwright smoke test launches the same controlled environment.

For the current Ubuntu 20.04 development environment, after installing `libgbm1`,
the Ubuntu 22.04 fallback Chromium build runs the smoke test successfully. Run
these commands from `frontend/` (the override is needed for both commands):

```sh
PLAYWRIGHT_HOST_PLATFORM_OVERRIDE=ubuntu22.04-x64 npx playwright install chromium
PLAYWRIGHT_HOST_PLATFORM_OVERRIDE=ubuntu22.04-x64 npm run test:browser
```

This is a local compatibility workaround; Playwright does not officially support
the detected Ubuntu 20.04 platform.

Implementation completion requires a successful `golangci-lint run ./...` using
the repository configuration, in addition to the relevant tests and frontend checks.

### Frontend formatting

From `frontend/`, run `npm run format` to apply Prettier or
`npm run format:check` to check formatting without changing files. The formatter
covers source, tests, and frontend configuration; generated output is excluded.
The frontend CI workflow checks formatting before building and running browser tests.

### Frontend theme

The Deep Space Explorer palette, typography, elevation, and Material UI component
overrides are centralized in `frontend/src/theme.ts`; Monaco's matching read-only
editor theme is defined in `frontend/src/YamlViewer.tsx`. Fontsource variable-font
packages bundle Space Grotesk, Geist, and JetBrains Mono into the frontend build,
so deployed pages make no runtime font or CDN requests. The visual references in
`frontend/stitch_cosmic_admin_ui_theme/` are intentionally excluded from formatting
and are not application or build inputs.

## Components

### Backend

> See [design](/backend/design.drawio)

#### Ingesting


#### REST-API

### Frontend

> See [design](/frontend/design.drawio)



## Aspirational

- Integration of MCP to allow agents to query the API 
