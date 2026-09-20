---
id: ADR-0011
title: Only the command package imports cobra
status: accepted
created: 2026-09-13
decided: 2026-09-13
backfilled: 2026-09-16
depends: []
updates: []
obsoletes: []
---

# ADR-0011: Only the command package imports cobra

## Context

The tool is a command-line program, and cobra provides its command tree, flags and help. Everything the tool does, reading a repository, checking it, rewriting a document, generating an index, is also something a program other than a command-line one might want to do.

## Decision

`cmd/archdoc` is the only package that imports cobra. Every package under `internal/` is usable without a terminal: nothing there prints, prompts, or calls `os.Exit`. A command is argument handling and nothing more, and a later desktop or TUI frontend calls the same packages directly.

## Alternatives

No alternatives were weighed.

## Consequences

Easier:

- A second frontend reuses everything below the command layer unchanged.
- A package under `internal/` is testable without capturing output or driving a terminal.

Harder:

- Exit codes and output have to travel from `internal/` to the command layer as values, an error type carrying the status and findings returned rather than printed.

Constrained:

- No package under `internal/` will import cobra, write to standard output, or terminate the process.
- Every command takes its writer from cobra, so that a test reads what a command wrote rather than capturing the process's streams.

## Sources

- `docs/ARCHDOC.md` at [9654fe2a], section Package layout, states that `cmd/archdoc` is the only package importing cobra and that every other package is usable without a terminal. This sets `created` and `decided`.
- `docs/DECISIONS.md` at [9654fe2d], sections Commands and interaction and Package layout and dependencies, record that exit codes travel as an error type and that nothing under `internal/` calls `os.Exit`.
- The decision was taken in a design session before that commit, not publicly available, and first written down on 2026-09-13. The date it was taken is not recalled.

[9654fe2a]: https://github.com/ollieread/archdoc/blob/9654fe2b9cdc3041294e99a64ebe6e61477e8bcc/docs/ARCHDOC.md
[9654fe2d]: https://github.com/ollieread/archdoc/blob/9654fe2b9cdc3041294e99a64ebe6e61477e8bcc/docs/DECISIONS.md
