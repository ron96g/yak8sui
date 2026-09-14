# Present Ready health and condition evidence in resource responses

Derive one presentation contract from each complete projected resource for list,
detail, and SSE responses. The contract includes every structurally readable
condition's type, status, reason, message, optional observed generation, and a
stale flag, plus Ready-based health. The first condition whose type is exactly
`Ready` determines health: `True` is Healthy, `False` is Unhealthy, and every
other or missing value is Unknown. Arbitrary malformed status remains available
in the complete read-only YAML and never implies health.

Health retains its reported mapping and carries `stale: true` when that Ready
condition has an integer `observedGeneration` lower than integer
`metadata.generation`. The UI labels this as Stale and shows the underlying
reported health and condition evidence rather than treating it as current
confirmation. Missing or non-integer generations make no freshness claim.
Condition-generation staleness remains separate from SSE connection and
observation state.
