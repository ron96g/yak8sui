# Index projected labels for ownership joins

Materialize each projected resource's string labels in a normalized `resource_labels` table maintained in the same transaction as the resource and tenant-membership rows. Resolve `labelJoin` candidates in SQLite, beginning with an indexed `(type_key, label_key, label_value, uid)` lookup and intersecting additional mappings with correlated indexed existence checks. Join tenant membership and resource identity through their existing covering indexes, apply namespace constraints in SQL, and use `LIMIT 2` for parent lookups because resolution needs only distinguish zero, one, or ambiguous matches.

This avoids decoding or scanning a tenant's complete type projection in the application for label ownership. It adds write amplification proportional to labels per changed resource and storage proportional to total projected labels. Existing databases receive a one-time guarded backfill from persisted JSON; subsequent writes maintain the normalized labels transactionally.

The initial sizing check uses 10,000 resources and parallel readers, matching ADR 0008's provisional resource scale. The SQLite connection pool remains serialized because the projection combines concurrent reads with informer writes on one embedded database; benchmark results are evidence for the provisional target, not a production SLA.
