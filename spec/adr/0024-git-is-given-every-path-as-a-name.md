---
id: ADR-0024
title: Git is given every path as a name
status: draft
created: 2026-11-02
decided:
depends: [ADR-0008]
updates: [ADR-0008]
obsoletes: []
---

# ADR-0024: Git is given every path as a name

## Context

[ADR-0008](0008-whether-a-frozen-document-changed-is-decided-by-git.md) decides whether a frozen document changed by asking `git diff`, with pathspecs anchored to the repository root and `diff.relative` forced off, so the answer does not depend on the directory the command was run from. The anchor is the `:(top)` prefix, which is itself pathspec magic. With `GIT_LITERAL_PATHSPECS` in the environment, git reads the prefix as part of the name and matches nothing. Without the anchor, git reads a path beginning with `:` as magic, and a repository's root may be a directory named that way. Either way no frozen document reads as changed, and L11 passes every edit.

The first was raised by an audit of the tool on 2026-10-26, recorded as [#25], and the second by review of its fix on 2026-11-02.

## Decision

Git runs from the repository root and is given every path as a name, relative to that root. `GIT_LITERAL_PATHSPECS=1` is set for every git command and any inherited pathspec setting is removed. No path carries a prefix, and `diff.relative` has nothing to resolve against.

## Alternatives

- **Anchoring each path with `:(top)` and forcing `diff.relative` off**, as ADR-0008 gives it. Lost because the anchor is magic that `GIT_LITERAL_PATHSPECS` turns off, and a path given without it is magic wherever it begins with `:`.

No other alternatives were weighed.

## Consequences

Easier:

- Whether a document changed will not depend on the directory, the inherited environment, or the characters in a path.

Harder:

- No git command will be able to use a pathspec feature, such as a glob.

Constrained:

- Every path given to git will be relative to the repository root.

[#25]: https://github.com/archdochq/archdoc/issues/25
