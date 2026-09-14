# Package the combined application with Kustomize and GHCR

Build the frontend and Go server in a multi-stage image, run the production
entry point as a non-root user in a minimal runtime with CA certificates, and
publish Linux amd64/arm64 images through GitHub Actions to GHCR. Pull requests
build without pushing; main and version-tag pushes publish commit-addressed and
human-readable tags using the workflow token. No cluster deployment is triggered
automatically by image publication.

Provide a Kustomize base and example namespace overlay. Deploy one replica with
Recreate updates, a persistent SQLite volume, a ClusterIP Service and in-cluster
ServiceAccount discovery/read permissions limited to explicitly selected sample
resource types. Keep real OIDC configuration in a separately supplied Secret.
Environment-specific ingress, TLS, image pull credentials, storage class, and
additional resource permissions belong in overlays.

Use TCP listener probes to avoid treating a partial resource-observation outage
as pod failure, consistent with ADR 0012. They establish listener availability
rather than deep health. SQLite and in-memory sessions currently preclude safe
multi-replica deployment; restarts require users to sign in again.

This packaging work follows the user's preference for Kubernetes/Kustomize and
GitHub/GHCR. PostgreSQL, requested Events, and actual live-cluster deployment are
still deferred; packaging does not mark the full integration ticket complete.
