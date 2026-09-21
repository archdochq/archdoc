---
id: ADR-0016
title: The scaffolded workflow uses the published actions
status: accepted
created: 2026-09-21
decided: 2026-09-21
depends: []
updates: []
obsoletes: []
---

# ADR-0016: The scaffolded workflow uses the published actions

## Context

Raised on 2026-09-21, when `archdochq/setup` and `archdochq/lint` were published.

The workflow `archdoc init` writes has carried a thirty-line shell block that resolves the release, downloads the archive and the checksum file, verifies one against the other, extracts the binary and installs it. Every repository ArchDoc has ever scaffolded holds a copy of that block, pinned to the release it was written for.

The block is correct, and it is correct because it was corrected twice: once to download outside the checkout, since `curl -O` writes through a symbolic link committed under the archive's name, and once to verify the archive before running it over a repository. Both corrections reached an existing repository only when its owner ran `archdoc update`.

It also cannot report a finding where a finding is read. `archdoc lint` prints to a log; turning its JSON into annotations against the lines that caused them is more shell than belongs in a file a tool writes into someone else's repository.

## Decision

The workflow `archdoc init` writes uses `archdochq/lint`, pinned to the version that scaffolded the repository, rather than installing `archdoc` itself. `archdoc index --check` follows as an ordinary step, because the action leaves the binary on `PATH`.

## Alternatives

- **Keeping the inline block.** It depends on `actions/checkout` and on GitHub's release hosting, and on nothing this project publishes, which is a real property for a repository whose policy forbids third-party actions. It was rejected because the two things it cannot do are the two that matter: deliver a fix without every owner running `archdoc update`, and put a finding on the line that caused it.
- **Publishing the actions and leaving the workflow alone.** The actions would serve people writing their own workflows while every scaffolded repository kept the block. That is two ways of doing one thing, and the one almost every repository would use is the one that cannot annotate.

## Consequences

Easier:

- A finding will appear on the line of the document that caused it, in the pull request, rather than in a log nobody opens.
- A fault in the install path will be fixed by moving one tag, reaching every repository that follows `@v1` without anyone running `archdoc update`.
- The scaffolded workflow will be a third of its former length, and will say what it does rather than how it fetches a binary.

Harder:

- Every scaffolded repository will depend on a second repository at CI time. One whose policy forbids third-party actions, or which vendors its CI, cannot use the generated workflow as written and will have to keep its own.
- The property that delivers a fix without `archdoc update` delivers a mistake the same way, because `@v1` moves.

Constrained:

- The agreement between the archive names `.goreleaser.yaml` publishes and the names the install path reconstructs will no longer be checkable in one place. Both sides were static text in this repository; one side now lives in another, so this repository pins the names it expects as literals and the action's own continuous integration checks the other side by downloading a real release.
- The version a workflow pins will be an input to an action rather than an environment variable, so anything reading that pin back reads it there. `archdoc update` did not, and rewrote a pinned repository to `latest` until it was taught the new shape.
- `archdoc update` will offer every existing repository the new workflow, replacing a block that works with one that rests on an action.
