# Compile the JSONPath subset into scoped SQLite traversal

Extend the versioned resource query with `jsonPath`, `jsonComparison`, and
`jsonValue`. `jsonComparison` is `exists` (the default) or `equals`; an Equals
value is exactly one JSON scalar, including explicit `null`, `false`, `0`, or
`""`. Keep this distinction in the browser rather than treating an empty input
as no comparison.

Use client-go's Apache-2.0 JSONPath parser as the mature syntax parser, followed
by a strict subset scanner that accepts root `$`, dot or quoted field access,
non-negative array indexes, and array wildcards only. Reject recursive descent,
predicates, scripts, slices, unions, object wildcards, and other nodes. Quoted
field access supports JSON string escaping and punctuation.

Compile every existence and scalar equality path into correlated SQLite
`json_each` predicates. Each path step checks its parent JSON type, so an array
wildcard never traverses object values. A final row proves existence even when
its type is `null`; equality checks SQLite's JSON type before its atom so booleans,
strings, integers, and null remain distinct. Correlated `EXISTS` avoids duplicate
resources when multiple wildcard nodes match. Field names, indexes, and values
are bind parameters; only fixed SQL fragments are assembled.

SQLite may erase the source spelling and precision of non-integer JSON numbers.
Those comparisons therefore use a bounded fallback over the already indexed
tenant/type candidate scope, decode numbers with `json.Number`, and compare their
exact rational values. Name and namespace fallback predicates run in the same
pass, and count/pagination remain after every filter. This compiled-filter
boundary is database-independent input plus an SQLite adapter; PostgreSQL is not
implemented in this decision.

Arbitrary JSON paths are not index-backed. The query first uses
`resource_tenant_scope (tenant_id,type_key,uid)` and the resource primary key,
then evaluates correlated JSON traversal. Add expression indexes only for
demonstrated production workloads.
