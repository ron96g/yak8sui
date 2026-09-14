# Deploy YAK8SUI to Kubernetes

The image serves the API and built frontend on port 8080 in one non-root process.
The current deployment uses SQLite on a persistent volume and **one replica**.
`Recreate` avoids overlapping writers during upgrades; upgrades have a brief
outage and users sign in again because sessions are held in memory.

## Image publishing

The `Container image` GitHub Actions workflow builds Linux amd64/arm64 images.
Pull requests and manual runs validate/build without publishing. Pushes to `main`
or `v*` tags publish to `ghcr.io/<owner>/<repo>` using `GITHUB_TOKEN`; no personal
access token is needed for publishing. For the intended repository this is
`ghcr.io/ron96g/yak8sui`.

Every push build gets `sha-<full-commit-sha>`. Main also gets `main`; semantic
version tags such as `v0.1.0` get `0.1.0` and `0.1`. Prefer a digest or SHA tag in
deployment overlays for reproducible rollouts. The example uses `main` for initial
evaluation. Images exist only after the workflow has successfully published them.

For local builds with a running Docker daemon:

```sh
docker build -t yak8sui:local .
```

## Configure your deployment

Prerequisites: a Kubernetes cluster, a default StorageClass (or a storage-class
patch), access to pull the GHCR image, and an OIDC client. The pod must reach the
Kubernetes API and OIDC provider over the network.

Start from `deploy/kustomize/overlays/example`. For an initial evaluation, adjust
its image tag and the base resource RBAC rules. For maintained environments,
create your own overlay referencing `../../base`, with a namespace, image digest,
storage settings, and RBAC patch. Use distinct ClusterRole/Binding names if
deploying multiple instances into the same cluster (for example, a name prefix).

Copy `deploy/config.example.yaml` to ignored `config/local.yaml`. Set the external
HTTPS URL, OIDC issuer/client/secret/callback, identity groups, tenant selectors,
and observed resource types. The callback is `<publicURL>/auth/callback`. Keep
the container paths for the frontend and SQLite. Omit kubeconfig to use the
ServiceAccount's in-cluster credentials.

The example observes selected `cert-manager.io` types only. They must exist in
the target cluster; missing configured types appear unavailable. It installs
neither cert-manager nor sample resources. Tenant selectors must match labels
already on the resources; an empty tenant view is expected when no labels match.

Keep `observation.groups` and the ClusterRole resource rules aligned. The base
grants only get/list/watch on the example resource types plus API discovery.
Add explicitly selected built-ins and configured relationships as needed. RBAC
does not enforce label selectors: the application enforces tenant visibility,
while its ServiceAccount can read all permitted resource types across namespaces.

Create the namespace and configuration Secret before starting the pod:

```sh
kubectl apply -f deploy/kustomize/overlays/example/namespace.yaml
kubectl -n yak8sui create secret generic yak8sui-config \
  --from-file=config.yaml=config/local.yaml --dry-run=client -o yaml | kubectl apply -f -
kubectl kustomize deploy/kustomize/overlays/example
kubectl apply -k deploy/kustomize/overlays/example
kubectl -n yak8sui rollout status deployment/yak8sui
```

The Secret contains the full YAML because the application currently reads its
OIDC secret from that configuration. Keep real configuration out of source control;
an external secret controller may instead manage `yak8sui-config` with the same
`config.yaml` key. No real secret is included in the manifests.

GHCR packages may initially be private. Either make the package public or create
a namespace-local registry credential with package read access and add it through
an `imagePullSecrets` patch. A private image without that configuration results
in `ImagePullBackOff`.

## Access and SSE

The base exposes a ClusterIP Service. Add an Ingress or Gateway appropriate to
your cluster, terminating TLS and forwarding to `yak8sui:80`. Configure its host
to match `server.publicURL` and register that exact OIDC callback. Forward both
API and frontend from the same origin. Disable response buffering for SSE and
allow long-lived connections; proxy idle/read timeouts must accommodate streams.
Controller-specific annotations belong in your environment overlay.

For loopback testing with a provider that permits HTTP localhost callbacks, set
`publicURL: http://127.0.0.1:20080`, the corresponding callback, and
`cookieSecure: false` in the Secret, then:

```sh
kubectl -n yak8sui port-forward service/yak8sui 20080:80
```

Open http://127.0.0.1:20080. Keep secure cookies enabled for HTTPS deployments.

## Operations

Configuration is restart-required. After updating the Secret:

```sh
kubectl -n yak8sui rollout restart deployment/yak8sui
kubectl -n yak8sui logs deployment/yak8sui
```

Startup/readiness/liveness use the HTTP listener's TCP port, not a resource's
sync state or an authenticated URL. OIDC discovery must succeed before the
listener starts; an unavailable resource type does not make healthy types
unusable. TCP probes verify listener availability, not full application health.

The read-only root filesystem has writable mounts only for SQLite and temporary
files. The pod filesystem group supplies PVC permissions; storage drivers must
support the configured ownership behavior. SQLite WAL requires a filesystem
with correct local locking semantics; choose a suitable block-backed volume.
Removing the overlay also deletes its PVC object: follow your volume reclaim and
backup policy before uninstalling. Do not scale this deployment above one replica;
PostgreSQL and multi-process session coordination are not yet delivered.

## Validation status

Local validation can render manifests with `kubectl kustomize` without a cluster.
Container execution and Kubernetes rollout additionally require a running build
daemon and cluster. See the implementation report for which checks were actually
executed; a successful render alone is not proof of a working rollout.

Initial packaging verification passed: actionlint v1.7.7 for the publishing
workflow, Kustomize rendering and kubeconform v0.6.7 strict validation (all seven
resources), CGO-disabled Linux amd64/arm64 production compilation, frontend
format/build checks, and repository golangci-lint. Docker image build/runtime and
live Kubernetes rollout remain unverified because the local Docker daemon and
selected cluster were unavailable. GHCR publication has not been executed.
