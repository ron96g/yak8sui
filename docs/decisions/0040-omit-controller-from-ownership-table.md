# Omit controller ownership from the table presentation

Remove the Controller column from the ownership table. Controller status describes an ownership edge rather than a resource, and a row-oriented table can make it appear to be a property of the resource.

The graph continues to show controller status on relationship connectors, where its edge-level meaning is explicit. The table remains focused on resource identity, hierarchy direction and depth, health, and actions. This supersedes the Controller-column requirement in ADR 0033 without changing the shared hierarchy state or relationship API.
