- use the `find-skills`.
- read the readme
- always document decision as ADRs under docs/decisions

## Implementation completion

After implementing or changing application code, run `golangci-lint run ./...`
from the repository root using the checked-in configuration. Fix reported issues
and require a successful run before marking the work complete. If execution is
blocked, record the blocker and leave verification incomplete. Run this alongside
the tests, typechecks, and browser checks appropriate to the change.

## Agent skills

### Issue tracker

Local Markdown. Before publishing or retrieving specs and tickets, read
`docs/agents/issue-tracker.md`.

### Triage labels

Use the five default triage roles. Before assigning triage status, read
`docs/agents/triage-labels.md`.

### Domain docs

Single-context layout. Before exploring or designing, read
`docs/agents/domain.md`.
