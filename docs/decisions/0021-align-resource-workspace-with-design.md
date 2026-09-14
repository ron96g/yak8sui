# Align implemented inspection with the frontend diagram

Use the frontend design diagram as the layout baseline: collapsible GroupVersion/Kind navigation alongside a filter area and resource table, with row selection opening a read-only Monaco YAML modal. Tenant selection moves to the toolbar so it remains accessible without displacing the resource workspace.

Render the single currently configured type from actual API metadata and expose resource creation time from the tenant-scoped projection. Present upcoming filters as explicitly disabled and condition health as available through YAML until their approved implementation slices arrive; do not simulate functioning filters or derived health. Keep read-only actions in accordance with ADR 0001 rather than the diagram's historical mutation controls.

Guard asynchronous resource responses by tenant context and detail request identity so switching tenants or closing the modal cannot restore stale contents.
