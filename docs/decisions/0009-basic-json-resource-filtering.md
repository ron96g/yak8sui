# Provide basic JSONPath resource filtering

The first increment includes a JSONPath field selector with an optional typed JSON equality comparison. Without a comparison, the filter checks path existence; combine this filter with Name and Namespace filters using AND.

Prefer an existing JSONPath library while exposing only field access, array indexes, and array wildcards initially. Equality succeeds when any selected value matches the typed comparison value; `true` differs from `"true"`. A present field containing `null` counts as existing, while a missing path does not.

Reject unsupported syntax explicitly. Embedded predicates, scripts, and recursive descent are deferred. Library selection remains an implementation decision.
