# Make ownership direction explicit in the explorer

Keep the ownership UI focused on the graph and actions: omit introductory headings and instructional prose. Retain contextual tooltips and actual incomplete/unavailable branch notices.

Show owners above and owned resources below the selected resource, joined by downward arrows and explicit statements such as “platform-settings owns platform-api.” Mark the selected resource distinctly and put controller ownership on the relationship connector, not on a node where it can be misread as a property of the resource.

Expand ancestor branches only upward and descendant branches only downward. This avoids repeating the immediate back-reference as a spurious cycle and keeps each chain readable. Retain path-based cycle protection, independently authorized lazy neighborhood streams, completeness notices, and related detail inspection.

Lay out siblings horizontally on the same ownership level, with child branches top-aligned and parent branches bottom-aligned. Keep each sibling level horizontally scrollable on narrow screens rather than stacking siblings into a misleading chain. Scrolling stays inside the ownership view.

Keep node cards compact (260px maximum width): resource name and kind are primary, the name opens inspection, and an icon toggles expansion. Additional group/version, namespace, and UID metadata is available on hover or keyboard focus. Short downward “owns” connectors retain controller labels and full accessible owner/child names, avoiding repeated long sentences and oversized cards across sibling rows.

Provide an “Expand entire hierarchy” action that recursively opens the existing authorized ancestor and descendant neighborhoods, including nodes discovered as branches load. Preserve cycle stops and incomplete/unavailable branch notices rather than claiming whole-cluster completeness. “Collapse branches” restores the selected resource's immediate neighborhood and releases deeper subscriptions; individual branch toggles remain usable after either action.

The controlled demo includes five resources across four levels: ClusterPolicy platform-guardrails owns ConfigMap platform-settings, which owns Widget platform-api, which owns ConfigMaps platform-runtime and platform-routes. This illustrates branching and traversal in both directions without changing the ownership resolver or tenant boundaries.
