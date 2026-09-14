# Use a controlled, loopback-only local demo for tickets 01–02

Provide `cmd/localdemo` as an explicitly non-production entry point that runs
the actual combined server, OIDC client and opaque sessions, tenant-scoped HTTP
APIs, SQLite latest-state projection, observer, and built browser UI. Supply a
controlled in-process OIDC provider and client-go fake list/watch source with
representative resources for two authorized tenants plus one out-of-tenant
resource. This demonstrates the implemented vertical slices without requiring
external credentials or a local Kubernetes control plane.

Bind the demo to loopback on ports 19080 and 19081 by default, separate from the
automated Playwright fixture ports. Persist only under the ignored
`.local/demo/` directory, with an explicit scoped reset command. Keep the
production `cmd/yak8sui` entry point unchanged for real OIDC and Kubernetes,
including its existing kubeconfig and in-cluster configuration behavior.

The fake client exercises the same informer and projection path but is not a
real Kubernetes deployment. Documentation must state that boundary and must not
present the controlled provider or its fixed demo secret as production-ready.
