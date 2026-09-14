# YAK8SUI resource inspection

YAK8SUI is a read-only administrative view of selected Kubernetes resources, isolated by explicitly authorized tenant context.

## Language

**Tenant**:
A configured security boundary with an identifier, display name, identity-provider groups, and a Kubernetes label selector.

**Authorized tenant**:
A tenant for which the signed-in user's verified identity-provider groups intersect the tenant's configured groups.

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
