# Require golangci-lint before implementation completion

Run `golangci-lint run ./...` from the repository root after application implementation changes, using the checked-in configuration. Resolve findings before marking work complete; if the tool cannot run, record the blocker and leave verification incomplete.

Keep this completion gate in `AGENTS.md` so every implementation agent follows it. Lint complements the behavioral tests, typechecks, and browser verification rather than replacing them.
