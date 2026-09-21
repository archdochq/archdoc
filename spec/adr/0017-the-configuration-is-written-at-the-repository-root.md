---
id: ADR-0017
title: The configuration is written at the repository root
status: accepted
created: 2026-09-21
decided: 2026-09-21
depends: []
updates: []
obsoletes: []
---

# ADR-0017: The configuration is written at the repository root

## Context

Raised on 2026-09-21, when ArchDoc's own `archdoc.json` was found in `spec/` with a `root` of `.` rather than at the root of the repository it documents. Nothing had decided that placement: `init` writes the configuration in the directory it is run in, and it had been run from `spec/`.

`root` exists so that a specification can live in a subdirectory of the project it documents. A configuration written below the repository root reaches that same end by a second mechanism, and `init` produces it by default, because running the command inside the directory meant to hold the documents is the obvious gesture.

Both shapes work. Every command except `init` finds its configuration by walking up from the working directory, so the file is located either way, and the generated workflow carries a `working-directory` naming the path from the repository root to the directory holding the configuration, which is empty for one shape and not the other.

## Decision

`archdoc init` writes `archdoc.json` at the root of the git repository it is run in, and sets `root` to the path from that root down to the directory it was run in. Outside a git repository there is nothing to anchor to, and the configuration is written in the current directory as before. A repository that already carries a configuration below its root keeps working unchanged: commands find it by walking up, and `archdoc update` still generates the workflow with the `working-directory` that shape needs.

## Alternatives

- **Leaving `init` to write the configuration where it is run.** Two placements reach the same end, and the one the tool produced by default is the one `root` exists to make unnecessary. Which mechanism a repository used could not be read from its layout without opening the configuration.
- **Refusing to run below the repository root.** Running `archdoc init` inside the directory meant to hold the specification is the obvious gesture, and refusing it teaches the user to run the command somewhere other than where they want the documents to land.
- **Moving an existing nested configuration on `update`.** `update` regenerates files the tool owns and leaves `archdoc.json` alone, and moving a tracked file out from under a repository that lints clean would be a change nobody asked for.

## Consequences

Easier:

- A repository's layout will be readable from its root: one `archdoc.json` there, with `root` naming where the documents sit.
- The generated workflow will carry no `working-directory`, because the directory holding the configuration and the repository root will be the same.

Harder:

- `init` will write two files outside the directory it is run in rather than one, and will print absolute paths for them.

Constrained:

- `working-directory`, and the git prefix it is computed from, will have to stay for repositories scaffolded before this, whose configuration sits below the root and whose workflow `update` still regenerates.
