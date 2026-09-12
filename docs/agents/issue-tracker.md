# Issue tracker: Local Markdown

Specs and tickets live in `.scratch/`.

## Conventions

- Feature directory: `.scratch/<feature-slug>/`.
- Spec: `spec.md` within that directory.
- Tickets: `issues/<NN>-<slug>.md`, one file per ticket, numbered
  from 01 in dependency order.
- Record triage state in a `Status:` line using the vocabulary in
  `docs/agents/triage-labels.md`.
- Record dependencies in `Blocked by:` using ticket numbers and titles.
- Append discussion under `## Comments`.

Publishing means writing these files. Fetching means reading the
referenced spec or ticket, including its comments.

## Wayfinding

- Map: `.scratch/<effort>/map.md`.
- Child tickets: `.scratch/<effort>/issues/<NN>-<slug>.md`.
- Record Type as research, prototype, grilling, or task.
- Wayfinding lifecycle uses Status: claimed or resolved.
- A ticket is unblocked when every listed blocker is resolved.
- Work the lowest-numbered open, unblocked, unclaimed ticket.
- Claim before working. On resolution, append an Answer, mark resolved,
  and add a summary and link to the map's Decisions-so-far section.

## Future repository

The intended future GitHub repository is
https://github.com/ron96g/yak8sui.

Local Markdown remains the active tracker until explicitly changed.
This setup does not configure a remote, publish GitHub issues, or push.
