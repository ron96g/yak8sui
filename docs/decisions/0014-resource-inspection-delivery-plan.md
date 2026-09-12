# Deliver resource inspection through twelve vertical slices

Publish the confirmed resource-inspection design as a local spec and twelve dependency-ordered, independently verifiable tickets. The initial inspection increment spans tickets 01–09; ownership and requested Events are tickets 10–11, followed by complete-workflow packaging and acceptance verification in ticket 12. Explicit blockers determine the execution frontier rather than a strictly sequential schedule.

Use the running application's HTTP/SSE interface as the primary integration-test boundary, backed by a controlled Kubernetes API and test identity provider. Run persistence-sensitive cases against SQLite and PostgreSQL, with a small browser acceptance suite for user interactions.

JSONPath library reuse concerns parsing and validation, not mandatory application-side evaluation. Ticket 07 is SQL-first: translate supported operations to native database predicates where semantics match and use scoped fallback only where needed, preserving ADR 0011's filtering-before-pagination contract.
