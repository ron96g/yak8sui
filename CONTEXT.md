# YAK8SUI resource inspection

YAK8SUI is an administrative view of selected Kubernetes resources, isolated by explicitly authorized tenant context. Inspection is read-only by default; optional per-tenant roles permit controlled mutations.

## Language

**Tenant**:
A configured security boundary with an identifier, display name, identity-provider groups, and a Kubernetes label selector.

**Authorized tenant**:
A tenant for which the signed-in user's verified identity-provider groups intersect the tenant's configured read-only, supervisor, or admin groups.

**Tenant role**:
An optional permission level derived from tenant-specific identity-provider groups. A supervisor may trigger reconciliation; an admin may trigger, update, and delete resources. Absence of either role remains read-only.

**Tenant context**:
The tenant identifier explicitly carried by an application request and independently authorized by the backend.

**Session**:
A bounded browser sign-in established from a verified OIDC identity and represented by an opaque server-managed cookie.

**Ownership relationship**:
A configured, directional permission to resolve a parent type and child type; it is not itself proof that two resources are related.

**Ownership edge**:
A relationship between two observed resources proven by a child owner reference whose API group, served version, kind, and UID identify the parent. Every endpoint independently requires both resources to belong to the active tenant.

**Ownership neighborhood**:
The bounded immediate parents and children of one authorized center resource. Further ownership-chain traversal occurs through explicit, lazy expansion of another neighborhood.

**Ownership hierarchy**:
The authorized, configured ownership paths assembled from loaded ownership neighborhoods. It is a partial view whose completeness is limited by expansion, observation availability, configuration, and tenant access.

**Ownership resolver**:
The configured interpretation that proves an ownership edge for one parent/child type pair. Supported interpretations may use Kubernetes owner references, a composite label join, or a structured field reference.

**Composite label join**:
A logical ownership rule in which every configured parent-label/child-label key pair is present and has equal values. It may dynamically bind a child to a replacement parent with the same labels.

**Field reference**:
A single structured reference in a child's spec or status that identifies its parent by API version, kind, name, UID, and namespace when required by scope.

**Ownership preview**:
The live read-only YAML inspection of a selected hierarchy node without changing the hierarchy's center resource, loaded branches, or presentation.
