# Preview related resource YAML without recentering ownership

Separate the ownership hierarchy's center resource from its preview selection. Activating a graph node or table row opens that node's read-only YAML in the current Ownership view while preserving the center, loaded branches, expansion state, and presentation; an explicit action remains available when the user intends to recenter and explore from that resource.

Use a dedicated tenant-authorized, UID-scoped detail replacement stream for the preview rather than deriving freshness from the center resource's type/page stream. The stream rechecks session, tenant membership, selected type, and observation freshness, emits complete current detail snapshots after committed changes, and reports observation loss or revoked access without retaining misleading content. Stale responses from a previous node, tenant, or center must not replace the active preview.

Present the hierarchy and YAML side by side where space permits and stack them on narrow screens. Reuse the existing read-only YAML viewer and expose loading, reconnecting, unavailable, and revoked states accessibly.
