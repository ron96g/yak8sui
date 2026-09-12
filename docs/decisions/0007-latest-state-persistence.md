# Persist a rebuildable latest-state projection

Support SQLite for single-instance deployments and PostgreSQL for shared deployments. Store the latest observed resource state as a rebuildable projection rather than retaining resource-version history.

Retain the last observed state across restarts, clearly marked stale until synchronized. Shared database support does not change the initial combined-process deployment decision in ADR 0006.

Tenant isolation takes precedence over offline inspection: after startup or observation loss, withhold resource contents on new reads until the relevant observation scope is synchronized. Already-open views become visibly stale and suspend updates; reconnecting revalidates access before refreshing.
