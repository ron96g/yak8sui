# Display live resource state with Ready-based health

Display the complete resource and a readable conditions view when available, with health derived from the condition whose type is `Ready`: `True` means Healthy, `False` means Unhealthy, and `Unknown` or a missing Ready condition means Unknown. Mark a condition Stale when its `observedGeneration` is older than the resource's `metadata.generation`; if `observedGeneration` is absent, use the reported condition without claiming generation freshness. Preserve the condition's reason and message.

Resource state updates automatically, with visible connection and freshness information so disconnected views do not silently appear current. Kubernetes Events remain request-only as decided in ADR 0001.

Apply the same Ready-based health rule to built-in resources initially; a missing Ready condition means Unknown. Kind-specific health evaluation is deferred.
