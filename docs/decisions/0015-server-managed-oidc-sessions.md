# Use opaque server-managed sessions for OIDC sign-in

The combined application completes Authorization Code OIDC with PKCE and stores only a cryptographically random session identifier in a Secure, HttpOnly, SameSite=Lax cookie. Session identity and expiry remain server-side so logout and expiry can immediately revoke requests and, through a session cancellation signal, later SSE streams; tenant identifiers remain explicit request inputs and are reauthorized against the verified groups on every request.
