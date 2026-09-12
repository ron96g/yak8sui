# Support extensible ownership traversal in both directions

The initial ownership relationship source is Kubernetes `metadata.ownerReferences`, and the view must show the entire chain upward to parents and downward to children. Relationship resolution must be extensible to future sources such as label selectors and references in `spec` or `status`.

Start with a simple YAML configuration describing the ownership hierarchy and explicitly configuring additional relationship resource types that may be observed. Show the full chain within the configured observation scope and indicate limits on completeness. Automatic discovery of the hierarchy during ingestion is a future enhancement.

YAML declares permitted relationships between resource types identified by API group and kind; actual object-to-object edges must be verified through owner references and resolved by UID. Future YAML configuration will select a specific resolver for each configured ownership relationship, such as a label-selector or spec/status-reference resolver.

Tenant isolation applies independently to every traversed resource, as decided in ADR 0002. Traversal stops at inaccessible resources without exposing their fetched contents; complete visible chains require consistent tenant labeling. Selecting an initial relationship source must not hard-code owner references as the only possible source.

Present an expandable hierarchy centered on the selected resource, with parents above and children below. Support multiple parents, identify controller-owner edges, guard against cycles, and load branches on demand while preserving navigation through the complete authorized chain.
