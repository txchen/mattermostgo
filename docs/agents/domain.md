# Domain Docs

This repo uses a single-context layout.

## Before exploring, read these

- `GLOSSARY.md` at the repo root: read the domain vocabulary.
- `docs/adr/`: read ADRs that touch the area you are about to work in.

If these files do not exist, proceed silently. Do not flag their absence or suggest creating them upfront. The `/domain-modeling` skill (also reached via `/grill-with-docs` and `/improve-codebase-architecture`) creates them lazily when terms or decisions actually get resolved.

## File structure

- `GLOSSARY.md`: shared domain vocabulary.
- `docs/adr/NNNN-<decision-slug>.md`: architecture decision records.

## Use the glossary’s vocabulary

When output names a domain concept (in an issue title, a refactor proposal, a hypothesis, or a test name), use the term defined in `GLOSSARY.md`. Avoid synonyms the glossary explicitly excludes.

If a concept is missing, reconsider whether the project uses it; note real vocabulary gaps for `/domain-modeling`.

## Flag ADR conflicts

If output contradicts an existing ADR, explicitly identify that ADR and explain why the decision should be reconsidered.
