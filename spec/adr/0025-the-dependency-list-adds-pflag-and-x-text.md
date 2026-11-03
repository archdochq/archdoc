---
id: ADR-0025
title: The dependency list adds pflag and x/text
status: proposed
created: 2026-11-03
decided:
depends: [ADR-0005]
updates: [ADR-0005]
obsoletes: []
---

# ADR-0025: The dependency list adds pflag and x/text

## Context

[ADR-0005](0005-the-dependency-list-is-closed.md) closes the dependency list at cobra, yaml.v3, huh, `x/term` and goreleaser. The code imports two modules the list does not name:

| Module | Imported by | For |
|---|---|---|
| `github.com/spf13/pflag` | `cmd/archdoc` | Walking each command's flags into the command reference. cobra gives a command's flags as a `*pflag.FlagSet`, and visiting them names pflag's types. |
| `golang.org/x/text` | `internal/changes` | Matching a configured `root` against git's listing of a revision in Unicode normalisation form C, through `unicode/norm`. |

cobra requires pflag, and huh requires x/text at the version the module names, so each was in the module graph before it was imported directly. pflag was imported with the command reference on 2026-10-26, and x/text on 2026-11-03, when each revision's documents came to be read under its own root. Neither was recorded against [ADR-0005](0005-the-dependency-list-is-closed.md). The pflag import was reported in [#36] on 2026-10-26, and the x/text import was found while correcting it on 2026-11-03.

## Decision

The dependency list is cobra for the command tree, pflag for its flags, yaml.v3 for front matter, huh for prompts, `x/term` to detect a terminal, x/text for Unicode normalisation, and goreleaser at build time only. Nothing else. A need that would take another library is met with the standard library or not met.

## Alternatives

No alternatives were weighed.

## Consequences

Easier:

- The command reference will read cobra's flags through the pflag types cobra gives them as.
- A root will match git's listing whichever normalisation form either is written in.

Harder:

- A repository adopting the tool will audit seven names rather than five.

Constrained:

- A need that would take another library will still be met with the standard library or not met.

[#36]: https://github.com/archdochq/archdoc/issues/36
