# Start with Custom Resource state and status

YAK8SUI must ultimately support inspecting Custom Resource state and status, diagnosing reconciliation problems, and exploring ownership relationships between Custom Resources. The first increment displays current Custom Resource state, including status, as the starting point toward that full scope.

Kubernetes Events are fetched only when explicitly requested. Subsequent decisions refine tenancy, discovery, presentation, and ownership below.

The scope being designed is read-only. Reconciliation diagnosis provides evidence through conditions, reasons/messages, resource YAML, ownership chains, requested Events, and health/freshness indicators. Agent-driven diagnosis is an aspirational future capability; mutation workflows are outside the current scope.
