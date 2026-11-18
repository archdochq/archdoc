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
│   ├── markdown/        a body's outline, as goldmark parses it
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

`internal/repo` opens a repository without failing on a malformed document: faults are recorded on the document as problems carrying a line and a message, and `lint` decides their severity. A document's headings, the sections they bound, every anchor, its links, references and `[[...]]` links, its prose text and the comment-masked text a section's emptiness reads are taken from the body's outline. Questions and a term's definition are read from one walk of the body's lines and its masked text, which also says whether a block is left open when the body ends. A body longer than 131,072 bytes is read by neither. A `Changelog`'s entries and a resolved question's answer are taken from the lines as written, within sections the outline's headings bound.

`internal/markdown` parses a body with goldmark into an outline of its blocks and inline pieces, each holding its position in the body as written, and is the only package that imports goldmark. `internal/repo` reads it for headings, links, `[[...]]` links, anchors, prose text and comment-masked text. Its tests compare the outline with cmark-gfm's parse of the cases under `testdata/commonmark`.

`internal/git` is an interface exposing `FilesAt`, `Changed`, `Differs`, `HasCommits`, `BranchExists`, `ListFiles`, `MergeBase`, `CurrentBranch`, `Prefix` and `RepoRoot`. It returns a sentinel error when git reports the directory is not inside a repository, and any other failure as an error. `FilesAt` reads many blobs through one `cat-file --batch`; `Changed` asks one `diff`, run from the repository root, which paths differ. Requests to `cat-file` are NUL-separated, and a missing path's answer is matched against the whole request. git runs with `LC_ALL=C` and without `GIT_DIR`, `GIT_WORK_TREE`, `GIT_COMMON_DIR` or `GIT_INDEX_FILE`, so the repository is found from the directory it runs in, and with `GIT_LITERAL_PATHSPECS=1` in place of any inherited pathspec setting, so a path is always a name. `ListFiles` lists a revision's whole tree by paths from the repository root, whichever directory git runs in. The repository root, existence and file listing are memoised per process. `Differs` compares files that moved by the object ids git computes for them. `PathOf` turns a path relative to the directory git was opened in into one from the repository root, as those operations take paths, asking git how it spells the directory, and joining it to `Prefix` where git cannot be run from it.

`internal/changes` reads every document at the merge base of HEAD and the configured branch in one `FilesAt` batch, and its `archdoc.json` in another, for the merge rules and `archdoc changes`. `internal/lint` imports it.

`internal/reference` builds the reference from the rule registry, `repo.Keys`, `config.Keys` and the export schema. The command tree is walked into it by `cmd/archdoc`.

`internal/repotest` builds throwaway repositories for tests and imports `testing`.

## Dependencies

- `github.com/spf13/cobra`
- `github.com/spf13/pflag`, which `cmd/archdoc` imports to walk cobra's flags into the command reference
- `gopkg.in/yaml.v3`
- `github.com/charmbracelet/huh`
- `github.com/charmbracelet/x/term`
- `golang.org/x/text`, which `internal/changes` imports for Unicode normalisation
- `github.com/yuin/goldmark/v2`, which `internal/markdown` imports to parse a body
- `github.com/goreleaser/goreleaser`, at build time only

Nothing else. `internal/repo` reads what the outline does not give it with the standard library.
