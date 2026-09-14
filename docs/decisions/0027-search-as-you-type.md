# Update resource search as the user types

Replace explicit Apply with automatic Name/Namespace filtering after a 300ms typing pause, including matching-mode changes. Reset to page one for filter edits, while pagination remains immediate. Keep backend filtering/counting before pagination and query-specific authorized SSE semantics from ADR 0026.

Cancel pending searches on tenant/type switches and unmount. Keep draft text independent of incoming SSE page updates so live changes cannot erase typing, and hide obsolete inline validation while a new draft is pending. Refresh retains the current query. Disable page navigation until pending filter edits are submitted.
