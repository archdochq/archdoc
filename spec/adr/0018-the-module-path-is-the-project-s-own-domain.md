---
id: ADR-0018
title: The module path is the project's own domain
status: accepted
created: 2026-09-21
decided: 2026-09-21
depends: []
updates: []
obsoletes: []
---

# ADR-0018: The module path is the project's own domain

## Context

Raised on 2026-09-21, when `archdoc.dev` became the project's site. The module path was `github.com/archdochq/archdoc`, which names the host the code sits on rather than the project. ArchDoc has already changed host once, from `ollieread` to `archdochq`, and a path naming the host changes with it.

Nothing imports these packages. ArchDoc is a command, so the path is read by people installing from source and by nothing else.

Go resolves a path that is not a known host by fetching it with `?go-get=1` and reading a `go-import` meta tag. For a command inside a module it asks for the full package path first and then shorter prefixes, so a tag served at the site's root answers for `archdoc.dev/cmd/archdoc` as well.

## Decision

The module path is `archdoc.dev`. The site serves a `go-import` meta tag naming the GitHub repository as the source, and the Go tool follows it to the code, which stays where it is.

## Alternatives

- **Keeping `github.com/archdochq/archdoc`.** The path names the host rather than the project, and a second change of host would change it again, as the move from `ollieread` did.
- **Serving the meta tag without changing the module path.** The Go tool checks that the fetched `go.mod` declares the path it was asked for, and fails on the mismatch. The tag alone resolves nothing.
- **Moving the command to the module root, so the path is `archdoc.dev` alone.** The command would then import cobra into the package every other package sits under, and the layout would be arranged around the install command rather than around the code.

## Consequences

Easier:

- Installing from source will name the project rather than its host: `go install archdoc.dev/cmd/archdoc@latest`.
- A later change of code host will change the meta tag the site serves, not the import path.

Harder:

- `go install github.com/archdochq/archdoc/cmd/archdoc@latest` will fail from the next release onwards, because the module will no longer declare that path.

Constrained:

- `archdoc.dev` will have to serve the `go-import` meta tag for as long as the tool is installable from source. Losing the domain, or the tag, breaks `go install` for every version declaring this path.
- Releases made before this declare the old path, and stay installable only as `github.com/archdochq/archdoc/cmd/archdoc@<tag>`.
