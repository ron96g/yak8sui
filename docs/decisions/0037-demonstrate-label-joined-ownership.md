# Demonstrate label-joined ownership with existing ConfigMap observation

Extend the controlled demo with a `ConfigMap` child named `platform-feature-flags` logically owned by `platform-settings`. Reuse the already observed core `ConfigMap` type for both endpoints and join two differently named label pairs, preserving the existing owner-reference hierarchy to demonstrate mixed resolver dispatch without broadening discovery or RBAC.

Expose a loopback-demo-only control that removes or restores one required child label. The open ownership neighborhood updates through the production observation and SSE path, demonstrating that composite joins require every mapping and that changing labels invalidates the logical edge.
