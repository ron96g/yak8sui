# Permit explicit unauthenticated cluster access

Support `identity.mode: allowAll` as an explicit alternative to the default `oidc` mode. In allow-all mode the production process skips OIDC discovery, accepts every request as one unbounded local identity, exposes every configured tenant, and permits every application mutation action. Tenant and resource label selectors remain enforced, while Kubernetes credentials and RBAC remain the final boundary on cluster reads and writes.

This mode exists for protected local or private-network operation against a real Kubernetes cluster. It must never activate because OIDC fields are missing: `oidc` remains the default and retains strict startup validation. The server logs a warning at startup, the session API identifies the active mode, and the frontend omits sign-in, expiry, and logout behavior because there is no revocable browser identity.

Allow-all deliberately supersedes the OIDC identity and group requirements in ADRs 0002 and 0015 only when explicitly configured. Any caller that can reach the HTTP service receives the Kubernetes authority granted to the application's kubeconfig or ServiceAccount, so deployment-level network access controls are required.
