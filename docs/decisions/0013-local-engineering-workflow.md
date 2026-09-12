# Use local Markdown for engineering specs and tickets

Use local Markdown under `.scratch/` as the active issue tracker, with one spec per feature and one file per implementation ticket. Retain the five default triage labels and explicit blocking dependencies so engineering skills can publish and retrieve work consistently before a hosted tracker is configured.

Use a single root domain glossary, created lazily, and retain `docs/decisions/` as the ADR location. Skill consumer rules live under `docs/agents/` and are referenced from `AGENTS.md`, the single authoritative root agent-instruction file. `CLAUDE.md` contains only the `@AGENTS.md` import so root instructions are maintained in one place.

The intended future repository is https://github.com/ron96g/yak8sui; adopting it as a tracker requires an explicit later decision. This setup does not configure a remote or push changes.
