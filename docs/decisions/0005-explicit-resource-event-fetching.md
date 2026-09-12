# Fetch resource Events when the Events tab is opened

Opening a resource's Events tab explicitly requests its Kubernetes Events, scoped to the resource UID. Fetch again on manual refresh, without an initial continuous Events subscription. Opening the resource detail view alone does not request Events.

Event requests enforce the same tenant authorization and independent resource membership checks as resource inspection.
