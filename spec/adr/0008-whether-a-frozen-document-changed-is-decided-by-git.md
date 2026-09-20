---
id: ADR-0008
title: Whether a frozen document changed is decided by git
status: accepted
created: 2026-09-13
decided: 2026-09-13
backfilled: 2026-09-16
depends: [ADR-0001]
updates: []
obsoletes: []
---

# ADR-0008: Whether a frozen document changed is decided by git

## Context

Under [ADR-0001](0001-terminal-documents-are-frozen.md) a terminal document is never modified, and the tooling has to report any change to one. That means comparing the document in the working tree against the document as it stands on the configured branch. git stores a blob and writes a file, and between the two it applies checkout filters: end-of-line normalisation under `core.autocrlf` or a `.gitattributes` rule, which is the default configuration on Windows. The bytes on disk and the bytes in the blob differ legitimately wherever such a filter is in force.

## Decision

Whether a frozen document has changed is decided by asking git, not by comparing bytes. The tooling asks `git diff` which of the frozen paths differ between the branch and the working tree, and reports exactly those. Pathspecs are anchored to the repository root and `diff.relative` is forced off, so the answer does not depend on the directory the command was run from.

## Alternatives

- **Comparing the stored blob against the file on disk byte for byte.** This was the first form. Under end-of-line normalisation every frozen document differed from its blob while git reported the tree clean, so the one rule the tool exists to enforce failed on every document in every repository on Windows.

## Consequences

Easier:

- The check agrees with what `git status` shows, on every platform and under every filter configuration.
- A repository that is clean to git is clean to the tool.

Harder:

- The check needs a git subprocess and a repository to run in. Outside a repository it cannot run and is skipped with a warning.

Constrained:

- Every question of the form "has this file changed since the branch" will be put to git rather than answered by reading bytes.
- The git interface will carry the flags that make the answer independent of the working directory.

## Sources

- `docs/DECISIONS.md` at [9654fe2d], section Linting, states the decision, the earlier byte comparison, and the Windows failure. This sets `created` and `decided`.
- `internal/git/git.go` at [9654fe2g], the docblock on `Changed`, states the same reasoning in the code.
- `docs/ARCHDOC.md` at [9654fe2a], rule L11, states the rule as the tool enforces it.
- The decision was taken in a design session before that commit, not publicly available, and first written down on 2026-09-13. The date it was taken is not recalled.

[9654fe2d]: https://github.com/ollieread/archdoc/blob/9654fe2b9cdc3041294e99a64ebe6e61477e8bcc/docs/DECISIONS.md
[9654fe2g]: https://github.com/ollieread/archdoc/blob/9654fe2b9cdc3041294e99a64ebe6e61477e8bcc/internal/git/git.go
[9654fe2a]: https://github.com/ollieread/archdoc/blob/9654fe2b9cdc3041294e99a64ebe6e61477e8bcc/docs/ARCHDOC.md
