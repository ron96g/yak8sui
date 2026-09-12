# Use Go and React with a combined initial backend

Use Go with Kubernetes client-go for discovery and informers, and React with TypeScript and Material UI for the frontend. Use Monaco for the read-only resource YAML view.

Initially deploy ingestion and API serving in one process, with internally separated responsibilities. Independently deployed ingestion and API replicas are a later extension.
