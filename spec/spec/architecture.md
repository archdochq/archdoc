---
title: Architecture
includes: [ADR-0005, ADR-0011]
---

# Architecture

## Packages

```
archdoc/
├── cmd/archdoc/         cobra commands; argument handling only
├── internal/
│   ├── config/          archdoc.json loading and defaults
│   ├── repo/            discovery, parsing, identifiers, derivation, writers
│   ├── lint/            rules, each a function returning findings
│   ├── lifecycle/       the transition graph and the transition gate
│   ├── index/           INDEX.md generation
│   ├── export/          the JSON shape and its published schema
│   ├── link/            wiki link resolution and suggestion
│   ├── glossary/        glossary editing
│   ├── git/             an interface with a shell-out implementation
│   ├── repotest/        throwaway repositories for tests
│   └── template/        embedded files, including agents/
├── plugin/              the guides packaged as skills
└── schema/              the published export schema
```

`cmd/archdoc` is the only package that imports cobra. No package under `internal/` prints, prompts, or calls `os.Exit`; each is usable without a terminal. Every command takes its writer from cobra.

`internal/repo` opens a repository without failing on a malformed document: faults are recorded on the document as problems carrying a line and a message, and `lint` decides their severity. A document's body is walked once, and every reader of it, sections, headings, links and the glossary, reads the same masked text.

`internal/git` is an interface exposing `FilesAt`, `Changed`, `HasCommits`, `BranchExists`, `ListFiles`, `CurrentBranch`, `Prefix` and `RepoRoot`. It returns a sentinel error when the directory is not inside a git repository. `FilesAt` reads many blobs through one `cat-file --batch`; `Changed` asks one `diff` which paths differ, with pathspecs anchored to the repository root and `diff.relative` off. Requests and responses are NUL-separated. The repository root, existence and file listing are memoised per process.

`internal/repotest` builds throwaway repositories for tests and imports `testing`.

## Dependencies

- `github.com/spf13/cobra`
- `gopkg.in/yaml.v3`
- `github.com/charmbracelet/huh`
- `github.com/charmbracelet/x/term`
- `github.com/goreleaser/goreleaser`, at build time only

Nothing else. Markdown is handled with the standard library.
