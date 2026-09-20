# Index all Kubernetes owner references

Normalize every observed resource's `metadata.ownerReferences` entries into a
SQLite table maintained in the same transaction as the resource projection. Index
the child type and referenced parent's API group, served version, kind, and UID so
downward ownership traversal begins with matching edges rather than loading and
decoding every tenant-authorized child document. Join the resulting child
identities to current resources and materialized tenant membership at read time;
an indexed reference proves identity but never grants access.

Store controller and non-controller references. Kubernetes permits at most one
controller reference, but YAK8SUI's ownership hierarchy intentionally supports all
owner references, multiple parents, and graph convergence. The controller flag is
retained as edge presentation metadata rather than used as an inclusion filter.
Starting with a controller-only projection would change existing behavior while
saving little implementation or storage complexity, and would require a later
backfill to restore non-controller edges.

Keep field references out of this table initially. Owner references have one
fixed Kubernetes location and can be projected independently of configuration;
field references use restart-required, rule-specific paths and must distinguish a
missing field from malformed present data. Field-reference downward lookup will
first use parameterized SQLite JSON traversal after indexed tenant/type scoping.
Introduce a configuration-dependent extracted-reference index only if benchmarks
at the provisional 10,000-resource target justify its migration, write
amplification, and reconciliation complexity.
