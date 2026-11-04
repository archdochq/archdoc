---
id: ADR-0024
title: Git is given every path as a name
status: accepted
created: 2026-11-02
decided: 2026-11-03
depends: [ADR-0008]
updates: [ADR-0008]
obsoletes: []
---

# ADR-0024: Git is given every path as a name

## Context

[ADR-0008](0008-whether-a-frozen-document-changed-is-decided-by-git.md) decides whether a frozen document changed by asking `git diff`, with pathspecs anchored to the repository root and `diff.relative` forced off, so the answer does not depend on the directory the command was run from. The anchor is the `:(top)` prefix, which is itself pathspec magic. With `GIT_LITERAL_PATHSPECS` set to 1 in the environment, git reads the prefix as part of the name and matches nothing. Without the anchor, git reads a path beginning with `:` as magic, and the `root` configured in `archdoc.json` may be a directory whose name begins with `:`. Either way no frozen document reads as changed, and L11 passes every edit.

The first was reported in [#25] on 2026-10-26, and the second on 2026-11-02, while the fix in [#41] was open.

## Decision

The git commands that decide whether a frozen document changed run from the repository root, and every path given to git is a name relative to that root. `GIT_LITERAL_PATHSPECS=1` is set for every git command and any inherited pathspec setting is removed. No path carries a prefix, and `diff.relative` has nothing to resolve against.

## Alternatives

- **Anchoring each path with `:(top)` and forcing `diff.relative` off**, as [ADR-0008](0008-whether-a-frozen-document-changed-is-decided-by-git.md) gives it. Lost because the anchor is magic that `GIT_LITERAL_PATHSPECS` turns off.
- **Passing `--literal-pathspecs` to the diff alone**, run from the repository root with plain paths, as [#25] gives it. Why the literal setting was given to every git command instead is not recorded.
- **Running the diff from the repository root with plain paths and no literal setting**, the first form of the fix. Lost because git reads a path beginning with `:` as magic, so a configured `root` named that way matched nothing.

No other alternatives were weighed.

## Consequences

Easier:

- Whether a document changed will not depend on the directory, the pathspec settings git inherits, or the characters in a path.

Harder:

- No git command will be able to use a pathspec feature, such as a glob.

Constrained:

- Every path given to git will be relative to the repository root.

[#25]: https://github.com/archdochq/archdoc/issues/25
[#41]: https://github.com/archdochq/archdoc/pull/41
