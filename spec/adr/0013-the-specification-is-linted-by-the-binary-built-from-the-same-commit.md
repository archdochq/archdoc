---
id: ADR-0013
title: The specification is linted by the binary built from the same commit
status: accepted
created: 2026-09-16
decided: 2026-09-20
depends: []
updates: []
obsoletes: []
---

# ADR-0013: The specification is linted by the binary built from the same commit

## Context

Raised on 2026-09-16, when `archdoc init` scaffolded this specification directory inside ArchDoc's own repository and wrote a workflow that downloads the release pinned as `ARCHDOC_VERSION` to lint it. That is the right workflow for every other repository, where the tool and the documents are versioned independently. Here they are not. The specification describes the tool at the commit it sits in. A change to a spec page and the code change it describes land together, and a downloaded release predates both.

## Decision

The workflow that checks this specification builds `archdoc` from the commit being checked and lints with that. The download step the tool generates is replaced, not supplemented.

## Alternatives

- **Downloading the pinned release, as generated.** The specification at a commit would be checked against a binary from an earlier one. A rule added and the page documenting it could not land in the same change, and a page describing a change in behaviour would be checked by the tool without it.

## Consequences

Easier:

- A spec page and the code it describes will be verified together, in the same run.
- A new lint rule and its documentation will land in one change.

Harder:

- `archdoc update` will offer to restore the download step every time it runs here, and the offer will have to be declined every time. The workflow is a permanent divergence from what the tool generates for this one repository.

Constrained:

- The workflow needs a Go toolchain on the runner, which no other scaffolded repository does.
- The workflow carries no `ARCHDOC_VERSION`, so `archdoc update` finds no pin here to read or raise.
