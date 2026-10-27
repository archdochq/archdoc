---
title: Architecture
includes: [ADR-0005, ADR-0011, RFC-0008, RFC-0009, RFC-0014, RFC-0015, RFC-0016]
---

# Architecture

## Packages

```
archdoc/
├── cmd/archdoc/         cobra commands; argument handling only
├── internal/
│   ├── config/          archdoc.json loading and defaults
│   ├── repo/            discovery, parsing, identifiers, derivation, writers, the transition graph
│   ├── lint/            rules, each a function returning findings
│   ├── changes/         the record on the configured branch, and what the working tree changes in it
│   ├── lifecycle/       the transition gate
│   ├── index/           INDEX.md generation
│   ├── export/          the JSON shape and its published schema
│   ├── reference/       ArchDoc's own reference and its published schema
│   ├── link/            wiki link resolution and suggestion
│   ├── glossary/        term files and GLOSSARY.md
│   ├── git/             an interface with a shell-out implementation
│   ├── repotest/        throwaway repositories for tests
│   └── template/        embedded files, including agents/
├── docs/guides/         the user guides a release's docs bundle carries
├── docs/build.go        the docs bundle's generator
├── install.sh           the install script a release attaches with its release set
├── plugin/              the skills repository's skeleton and generator
└── schema/              the published export and reference schemas
```

`cmd/archdoc` is the only package that imports cobra. No package under `internal/` prints, prompts, or calls `os.Exit`; each is usable without a terminal. Every command takes its writer from cobra.

`internal/repo` opens a repository without failing on a malformed document: faults are recorded on the document as problems carrying a line and a message, and `lint` decides their severity. A document's body is walked once, and every reader of it, sections, headings, links and the glossary, reads the same masked text. A `Changelog`'s entries and a resolved question's answer are taken from the lines as written, within sections found in the masked text.

`internal/git` is an interface exposing `FilesAt`, `Changed`, `HasCommits`, `BranchExists`, `ListFiles`, `MergeBase`, `CurrentBranch`, `Prefix` and `RepoRoot`. It returns a sentinel error when git reports the directory is not inside a repository, and any other failure as an error. `FilesAt` reads many blobs through one `cat-file --batch`; `Changed` asks one `diff`, run from the repository root, which paths differ. Requests to `cat-file` are NUL-separated, and a missing path's answer is matched against the whole request. git runs with `LC_ALL=C` and without `GIT_DIR`, `GIT_WORK_TREE`, `GIT_COMMON_DIR` or `GIT_INDEX_FILE`, so the repository is found from the directory it runs in. The repository root, existence and file listing are memoised per process. `PathOf` turns a path relative to the directory git was opened in into one from the repository root, as those operations take paths, by joining it to `Prefix`.

`internal/changes` reads every document at the merge base of HEAD and the configured branch, and its `strict`, in one `FilesAt` batch, for the merge rules and `archdoc changes`. `internal/lint` imports it.

`internal/reference` builds the reference from the rule registry, `repo.Keys`, `config.Keys` and the export schema. The command tree is walked into it by `cmd/archdoc`.

`internal/repotest` builds throwaway repositories for tests and imports `testing`.

## Dependencies

- `github.com/spf13/cobra`
- `gopkg.in/yaml.v3`
- `github.com/charmbracelet/huh`
- `github.com/charmbracelet/x/term`
- `github.com/goreleaser/goreleaser`, at build time only

Nothing else. Markdown is handled with the standard library.
