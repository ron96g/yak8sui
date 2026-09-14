# 07: Filter by resource JSON values using SQL-first execution

**What to build:** Tenant users apply basic JSONPath existence or typed equality filters alongside Name and Namespace filters, with native database execution wherever the agreed semantics can be preserved.

**Blocked by:** 06 — Search resource lists before pagination.

**Status:** resolved

## Acceptance criteria

- [ ] Reuse an established parser/validator where suitable and accept only field access, array indexes, and array wildcards; reject unsupported predicates, scripts, recursive descent, and invalid syntax clearly.
- [ ] With no comparison, selected-path existence matches even a present null; missing paths do not match. The UI can distinguish absent comparison from an explicit JSON null comparison.
- [ ] An explicit typed JSON value matches if any selected value is equal; booleans, numbers, strings, and null are not conflated. Multiple wildcard results are all eligible to match.
- [ ] Translate supported operations to native SQLite JSON predicates/traversal wherever equivalent semantics are available. Parser/library use must not force all JSONPath evaluation into application memory.
- [ ] Native and fallback execution share the same subset and typed matching semantics, with no SQL-injection opportunity from paths or values. Scoped fallback is used only where needed for faithful execution.
- [ ] AND JSON filtering with Name and Namespace before pagination and count calculation, including matches beyond the first candidate batch.
- [ ] Public API tests exercise native execution and any fallback for field/index/wildcard paths, null versus missing, typed comparisons, multiple matching values, malformed input, combined filters, tenant isolation, and page/count correctness.
- [ ] Document actual SQL translations, fallback cases, and applicable indexing limits. Leave a clear adapter boundary for the PostgreSQL implementation in ticket 08.

## Implementation context

Use ADRs 0009, 0011, and 0014. Parsing location, evaluation location, and index usage are three separate concerns. Prefer existing libraries and best-effort native SQL without weakening semantics to fit an optimization.

## Comments

- 2026-09-13: Claimed for implementation. The agreed public seams are the versioned HTTP/SSE resource query contract and browser resource workspace; persistence-sensitive behavior will run against the real SQLite projection.
- 2026-09-13: Resolved. Added strict client-go-parsed field/index/array-wildcard JSONPath filters, explicit Exists/Equals typed scalar semantics, native parameterized SQLite `json_each` traversal with exact-number scoped fallback, shared REST/SSE behavior, automatic browser search and live entry/exit coverage. Verified race tests, mandatory lint, frontend formatting/typecheck/build, Chromium browser and isolated demo suites; the rebuilt local demo is running at http://127.0.0.1:19080 (PID 231091).

## Answer

Ticket 07 is implemented as the complete SQLite/API/SSE/browser vertical slice. Decisions and limitations are recorded in ADR 0028 and usage examples are in README.md. Ticket 08 PostgreSQL work was not started.
