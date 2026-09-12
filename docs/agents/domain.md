# Domain docs

## Layout

This is a single-context project:

- `CONTEXT.md`: root domain glossary.
- `docs/decisions/`: architecture decision records.

## Consumer rules

Before exploring or designing, read the glossary if present and the
ADRs relevant to the work. Proceed silently when the glossary is absent;
create it lazily when domain terms are resolved.

Use glossary terminology in specs, tickets, code, and tests.
Surface conflicts with an existing ADR explicitly before changing
the agreed design.

Record decisions as sequentially numbered ADRs under `docs/decisions/`,
following the existing repository convention.
