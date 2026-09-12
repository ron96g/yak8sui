# Keep synchronized resource types available during partial observation failure

When a configured resource type cannot be synchronized, keep healthy types available while marking the affected type unavailable and withholding its cached contents. Mark dependent hierarchy branches incomplete and expose the failure through configuration/status information rather than presenting an empty list as authoritative.

A valid configuration with an unavailable dependency is distinct from malformed configuration that prevents startup. This refines the observation-freshness rule in ADR 0007.
