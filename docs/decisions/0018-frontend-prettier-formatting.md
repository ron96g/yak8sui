# Format the frontend with Prettier

Use a pinned local Prettier development dependency and npm scripts for writing and checking frontend formatting. Preserve the prevailing single-quote, no-semicolon style while letting Prettier determine layout consistently across TypeScript, TSX, HTML, and configuration files.

Exclude generated artifacts, the npm-managed lockfile, and the design diagram from formatting. Require the read-only formatting check in the existing frontend CI workflow, with commands running from the frontend directory and its Go-backed browser fixture prerequisites available.
