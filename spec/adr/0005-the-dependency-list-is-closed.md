---
id: ADR-0005
title: The dependency list is closed
status: accepted
created: 2026-09-13
decided: 2026-09-13
backfilled: 2026-09-16
depends: []
updates: []
obsoletes: []
---

# ADR-0005: The dependency list is closed

## Context

The tool is a single binary that reads and rewrites Markdown files with YAML front matter, prompts at a terminal, and shells out to git. Each of those needs is met by one library, and each library added is code the tool ships, code that can change under it, and a reason for a repository's security policy to look twice.

## Decision

The dependency list is cobra for the command tree, yaml.v3 for front matter, huh for prompts, `x/term` to detect a terminal, and goreleaser at build time only. Nothing else. A need that would take another library is met with the standard library or not met.

## Alternatives

No alternatives were weighed.

## Consequences

Easier:

- The binary carries little that is not its own, and an upgrade to it is an upgrade to little else.
- A repository adopting the tool audits five names.

Harder:

- A capability that a library would provide in a line has to be written, or refused. Taken from what followed rather than anticipated: on 2026-09-14 the export's published schema was checked against its structs by reflection rather than by validating output against it, because every JSON Schema validator would have been a sixth dependency.

Constrained:

- Markdown will be handled with the standard library.
- Any test-only convenience that needs a library is refused on the same terms as production code.

## Sources

- `docs/ARCHDOC.md` at [9654fe2a], section Dependencies, lists the five and states "Nothing else." This sets `created` and `decided`.
- `docs/DECISIONS.md` at [9654fe2d], section Package layout and dependencies, records the same list.
- `docs/ARCHDOC.md` at [72df287a], section Dependencies, records the refusal of a JSON Schema validator on 2026-09-14, cited here as a consequence recorded later.
- The decision was taken in a design session before the first commit, not publicly available, and first written down on 2026-09-13. The date it was taken is not recalled.

[9654fe2a]: https://github.com/ollieread/archdoc/blob/9654fe2b9cdc3041294e99a64ebe6e61477e8bcc/docs/ARCHDOC.md
[9654fe2d]: https://github.com/ollieread/archdoc/blob/9654fe2b9cdc3041294e99a64ebe6e61477e8bcc/docs/DECISIONS.md
[72df287a]: https://github.com/ollieread/archdoc/blob/72df287c4fdbc57afb1cb03775b1f4a0f4f99da8/docs/ARCHDOC.md
