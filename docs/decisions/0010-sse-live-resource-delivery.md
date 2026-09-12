# Deliver live resource views through SSE

Use Server-Sent Events for server-to-browser resource updates, starting with a snapshot followed by updates. On reconnect, reauthorize the tenant and replace the view with a fresh snapshot before resuming updates rather than replaying every missed change. Display disconnected and resynchronizing states.

Prefer existing protocol implementations or libraries where suitable. Snapshot/update ordering and authorization remain application responsibilities; no specific SSE dependency has been selected.

Serve the frontend and API from the same origin, with backend-managed OIDC login and secure HTTP-only session cookies. Logout or session expiry closes live streams and clears displayed tenant data. Revalidate authorization when establishing a stream and bound existing streams by session lifetime.
