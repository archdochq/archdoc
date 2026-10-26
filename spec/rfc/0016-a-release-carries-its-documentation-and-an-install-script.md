---
id: RFC-0016
title: A release carries its documentation and an install script
status: proposed
created: 2026-10-19
decided:
depends: [RFC-0014, ADR-0005, ADR-0006, ADR-0019]
updates: []
obsoletes: []
---

# RFC-0016: A release carries its documentation and an install script

## Abstract

Every release attaches a docs bundle, `archdoc_<version>_docs.tar.gz`, holding user guides written under `docs/guides/`, the reference [RFC-0014](0014-lint-rules-and-the-command-reference-are-published-as-data.md) defines, `PROCESS.md`, the published schemas and a manifest, and an `install.sh` that installs that release on Linux and macOS. Both are built by goreleaser before it publishes and are listed in `checksums.txt`. Anything displaying the documentation reads both from the release by its tag.

## Motivation

A person learning ArchDoc has `README.md`, one page covering installing, getting started, the pre-commit hooks, the commands and the document types, and `PROCESS.md`, the rules of the process. The guides under `internal/template/agents/` are written for coding agents to follow, and `docs/` describes the code and the export. Nothing describes for a person how to backfill, how to keep a spec repository beside the code, how to check a repository in CI other than GitHub's, or how to contribute by pull request. Documentation written apart from the code describes whatever the code did when it was written, not the release a reader installed.

Anything that displays ArchDoc's documentation, such as a site documenting the tool, needs the documentation of each release as files that release published. Today a release attaches the archives and `checksums.txt`, and nothing else.

Installing ArchDoc without Homebrew, `go install` or GitHub Actions means downloading the archive for the platform, checking it against `checksums.txt` and extracting the binary by hand. `archdochq/setup` performs those steps, but only inside a GitHub Actions job. A Dockerfile, a GitLab job or a Linux machine without Homebrew has no equivalent.

## Proposal

### Concepts

| Concept | Meaning |
|---|---|
| User guide | A Markdown page under `docs/guides/` describing, for a person using ArchDoc, the behaviour of the release that carries it |
| Contents | `docs/guides/contents.json`: the guides in sections, in order, with titles |
| Docs bundle | `archdoc_<version>_docs.tar.gz`, attached to every release |
| Manifest | `manifest.json` in the docs bundle, describing the bundle |
| Generator | `docs/build.go`, which writes the docs bundle and the rendered install script |
| Install script | `install.sh`, which downloads, verifies and installs one release's binary |

### Components

| Component | Responsibility |
|---|---|
| `docs/guides/` | The user guides and their contents |
| `docs/build.go` | The generator |
| `install.sh` | The install script, at the repository root, with no release set |
| `.goreleaser.yaml` | A `before` hook running the generator, and `extra_files` attaching its output to the release and listing it in `checksums.txt` |
| `.gitignore` | `build/`, where the generator writes |
| `cmd/archdoc/release_test.go` | Runs the generator and the rendered install script, and checks that `.goreleaser.yaml` runs and attaches the generator's output |
| `README.md` | The install script's line under Installing |
| Spec pages | `release.md` for the bundle and the install script; `architecture.md` for `docs/guides/`, `docs/build.go` and `install.sh` in the tree |

### The user guides

The guides are Markdown files directly under `docs/guides/`, with no front matter. A guide changes in the same change as the behaviour it describes, so the guides a release carries describe that release. A relative link in a guide is a listed guide's file name exactly as the contents give it, optionally followed by a fragment, or is a fragment alone; anything else is linked by absolute URL. The links are those `internal/repo` reads from a document's body: inline links, images and link reference definitions, outside code spans, fenced blocks and comments. A fragment is not checked against the target's headings. They are distinct from the guides under `internal/template/agents/`, which are written for coding agents to follow, and from the rest of `docs/`, which describes the code.

The first guides:

| Section | File | Title |
|---|---|---|
| Start | `installing.md` | Installing ArchDoc |
| Start | `five-minutes.md` | Five minutes with ArchDoc |
| Concepts | `concepts.md` | Concepts |
| Guides | `backfilling.md` | Backfilling |
| Guides | `beside-the-code.md` | A spec repository beside the code |
| Guides | `ci.md` | Checking in CI: GitHub, GitLab and pre-commit |
| Guides | `coding-agents.md` | Working with coding agents |
| Guides | `pull-requests.md` | Contributing by pull request |

`docs/guides/contents.json` holds the sections in order, each with a title and its guides in order, each guide with its file and title:

```json
{
  "sections": [
    {"title": "Start", "guides": [
      {"path": "installing.md", "title": "Installing ArchDoc"},
      {"path": "five-minutes.md", "title": "Five minutes with ArchDoc"}
    ]},
    {"title": "Concepts", "guides": [
      {"path": "concepts.md", "title": "Concepts"}
    ]}
  ]
}
```

| Rule | Checked by |
|---|---|
| Every `.md` file under `docs/guides/` is listed exactly once | The generator |
| Every listed path names a `.md` file directly under `docs/guides/` | The generator |
| Every section has a title and at least one guide, and every guide a title | The generator |
| Every relative link in a guide is a listed guide's file name, with or without a fragment, or a fragment alone | The generator |

### The docs bundle

`archdoc_<version>_docs.tar.gz`, where `<version>` is the tag less its leading `v`, as in the archives' names. Its entries sit at the root of the archive, with no enclosing directory, and it holds files only, with no directory entries:

```
manifest.json
reference.json
PROCESS.md
guides/<file>.md        one per listed guide, in the contents' order
schema/<type>/v<n>.json one per published schema, in path order
```

| Entry | Content |
|---|---|
| `manifest.json` | Below |
| `reference.json` | What `archdoc reference` prints for the release |
| `PROCESS.md` | The copy `init` writes, embedded by `internal/template` |
| `guides/` | The listed guides, byte for byte |
| `schema/` | Every published schema, byte for byte from the repository's `schema/` directory, at the same paths |

`manifest.json`:

```json
{
  "bundle": 1,
  "version": "v0.6.0",
  "commit": "949c1a7…",
  "schemas": {"export": 3, "reference": 1},
  "sections": [
    {"title": "Start", "guides": [
      {"path": "guides/installing.md", "title": "Installing ArchDoc"}
    ]}
  ]
}
```

| Field | Content |
|---|---|
| `bundle` | `1`, the bundle's own number |
| `version` | The tag, as `archdoc --version` prints it |
| `commit` | The full commit the release was built from |
| `schemas` | The number of the export schema the release emits, from `export_schema.properties.schema.const` in `reference.json`, and the reference's, from its `schema` |
| `sections` | The contents, with each path relative to the bundle's root |

Which guides and schemas a bundle holds varies by release and is not part of its shape; `manifest.json`, `reference.json`, `PROCESS.md`, `guides/` and `schema/` are. The bundle's number follows the rule [ADR-0019](../adr/0019-export-stability.md) sets for the export's: before ArchDoc 1.0, a change that removes one of those entries or a field of the manifest, or changes one's type or meaning, moves it to the next integer, at most once between two tagged releases, pre-releases included; a number a tagged release has carried is never given to another shape; from 1.0 the bundle only gains entries and optional fields.

### `docs/build.go`

`docs/build.go` is a `main` package excluded from every build by a `//go:build ignore` constraint, as `plugin/build.go` is. It is run from the repository root as `go run ./docs/build.go [-guides <dir>] <directory> <tag> <commit>`, where `-guides` names the guides' directory in place of `docs/guides`, and uses the standard library, `internal/template`, `internal/repo` to read the guides' links, and a binary it compiles from the same checkout. It reads everything it writes before changing anything, so a failure leaves `<directory>` as it was.

1. Exit non-zero unless there are exactly three arguments besides `-guides`, none empty, and `<directory>` is missing or holds nothing but files named `archdoc_*_docs.tar.gz` and `install.sh`.
2. Read the contents and the guides, and exit non-zero unless they pass the rules above.
3. Run `go run -ldflags "-X main.version=<tag>" ./cmd/archdoc reference`, exiting non-zero if it fails, and read the two numbers from what it prints.
4. Read `PROCESS.md` from `internal/template`, every file under `schema/`, and `install.sh`, exiting non-zero unless `install.sh` holds the line `release=` exactly once.
5. Create `<directory>` if it is missing, and remove the files it holds otherwise.
6. Write the bundle into `<directory>` with `archive/tar` and `compress/gzip`, its entries in the order above.
7. Write `install.sh` into `<directory>` with the line `release=` replaced by `release=<tag>`, mode `0755`.

### Building and attaching

goreleaser runs the generator as a `before` hook, after its own check that the tree is clean and before it builds, writes the release notes or computes checksums:

```yaml
before:
  hooks:
    - go mod tidy
    - go run ./docs/build.go build/docs {{ .Tag }} {{ .FullCommit }}

release:
  extra_files:
    - glob: build/docs/*

checksum:
  extra_files:
    - glob: build/docs/*
```

The output is written outside `dist/`, which goreleaser requires to be empty after its hooks run. The bundle and `install.sh` are attached to every release, a pre-release included, and `checksums.txt` lists each under its file name beside the archives. A generator that exits non-zero stops the release before anything is published, leaving the tag pushed and unpublished.

### `install.sh`

A POSIX shell script, so it runs under `sh` on a system without `bash`. The repository's copy sets no release; the copy attached to a release installs that release by default. The copy the latest stable release attaches is at `https://github.com/archdochq/archdoc/releases/latest/download/install.sh`, and anything serving the script, such as a site documenting the tool, copies it from a release.

```sh
curl -fsSL https://github.com/archdochq/archdoc/releases/latest/download/install.sh | sh
curl -fsSL https://github.com/archdochq/archdoc/releases/latest/download/install.sh | ARCHDOC_VERSION=v0.6.0 sh
```

| Variable | Effect |
|---|---|
| `ARCHDOC_VERSION` | The release to install, as a tag such as `v0.6.0`, in place of the one the script was attached to |
| `ARCHDOC_INSTALL_DIR` | The directory to install into |
| `ARCHDOC_DOWNLOAD_URL` | The base URL of the release's assets, in place of `https://github.com/archdochq/archdoc/releases/download/<tag>` |

1. Take the release from `ARCHDOC_VERSION`, or else from `release`, and exit non-zero if neither is set.
2. Map `uname -s` to `linux` or `darwin` and `uname -m` to `amd64` (`x86_64`, `amd64`) or `arm64` (`aarch64`, `arm64`), exiting non-zero on anything else.
3. Download `archdoc_<release less v>_<os>_<arch>.tar.gz` and `checksums.txt` into a temporary directory, with `curl`, or `wget` where `curl` is missing.
4. Check the archive against the line in `checksums.txt` ending in its name, with `sha256sum`, or `shasum -a 256` where that is missing.
5. Extract `archdoc` from the archive's root, copy it, mode `0755`, to a temporary name in the install directory, and rename it to `archdoc` there, so a binary already installed is replaced whole or not at all. The install directory is `ARCHDOC_INSTALL_DIR`, or else `/usr/local/bin` where it is writable, or else `$HOME/.local/bin` where `HOME` is set, and is created when missing. An `archdoc` there that is a symbolic link, as a package manager such as Homebrew installs it, is not replaced.
6. Print the release and the path installed, and a warning when the directory is not on `PATH`.

On exit, whether the script succeeds, fails or is interrupted, it removes the temporary directory and any temporary file it left in the install directory. Nothing is installed unless every step before the rename succeeds; an install directory the script created is left in place.

### Tests

| Test | Checks |
|---|---|
| The generator, run with `go run` from the repository root with a missing argument, or with `-guides` naming a temporary directory holding each fault the Errors table lists for the contents and guides | Exits non-zero and leaves the output directory as it was |
| The generator, run with `go run` from the repository root into a temporary directory with a fixed tag and commit | `archdoc_<version>_docs.tar.gz` and `install.sh` are written; the bundle holds exactly the entries above; `manifest.json` carries the tag, the commit, the contents and the numbers `reference.json` gives; `PROCESS.md` is the embedded copy; `install.sh` sets `release` to the tag |
| The rendered `install.sh`, run under `sh` against a local server holding an archive named from `.goreleaser.yaml` for the test's own platform and a `checksums.txt` | The binary is installed into `ARCHDOC_INSTALL_DIR`; with a wrong digest, or no line for the archive, the script exits non-zero, installs nothing and leaves no temporary file in the install directory. Skipped on a platform the script does not support |
| `.goreleaser.yaml` attaches the generator's output | The `before` hook runs `docs/build.go` into the directory that `release` and `checksum` both list in `extra_files` |

The install script's test renders the archive's name from `.goreleaser.yaml` as the test of `archdochq/setup`'s names does, so a rename the script does not follow fails before a release.

### Errors

| Condition | Result |
|---|---|
| The generator's arguments are missing or empty, or `<directory>` holds a file it did not write | The generator exits non-zero before writing anything |
| A guide is missing from the contents, listed twice, or listed but absent; a section or guide has no title; a section is empty; a listed path is not a `.md` file; a relative link names no listed guide | The generator exits non-zero before writing anything. The generator's test fails, in CI and in the release workflow's test step; goreleaser's hook would stop the release before it is published |
| `archdoc reference` fails | As above |
| `install.sh` holds no line `release=`, or more than one | As above |
| `install.sh` with no release set and no `ARCHDOC_VERSION` | Exits non-zero, naming the variable |
| `install.sh` on another operating system or architecture | Exits non-zero, naming the platform |
| `install.sh` without `curl` or `wget`, or without `sha256sum` or `shasum` | Exits non-zero, naming what is missing |
| The archive or `checksums.txt` cannot be downloaded, `checksums.txt` has no line for the archive, or the archive does not match | Exits non-zero, having installed nothing |
| The install directory cannot be created or written, or no directory applies because `/usr/local/bin` is not writable and `HOME` is unset | Exits non-zero, naming the directory where there is one, and `ARCHDOC_INSTALL_DIR` |
| An `archdoc` in the install directory is a symbolic link | Exits non-zero, naming the link and `ARCHDOC_INSTALL_DIR` |

### Out of scope

- **The release notes in the bundle.** goreleaser writes them after its `before` hooks run, so they do not exist when the bundle is built. They are on the release, which GitHub serves by tag.
- **Signing the archives or attesting how they were built.** `checksums.txt` comes from the same release as the archive, so it shows that an archive arrived intact, not who built it. Signing changes every asset a release publishes, not the install script alone.
- **An install script for Windows.** It would be a PowerShell script, a design of its own. A Windows user has the release's zip archive and `go install`.

## Alternatives considered

The design rests on [RFC-0014](0014-lint-rules-and-the-command-reference-are-published-as-data.md), whose reference the bundle carries; on [ADR-0005](../adr/0005-the-dependency-list-is-closed.md), under which the generator uses the standard library rather than a tool beside goreleaser; on [ADR-0006](../adr/0006-markdown-is-handled-with-the-standard-library.md), under which the guides' links are read without a Markdown parser; and on [ADR-0019](../adr/0019-export-stability.md), whose rule for the export's number the bundle's follows.

- **Building the bundle in a job after the release is published.** The bundle could carry the release notes. Lost because `checksums.txt` is published by then, and adding the bundle to it would mean replacing the file `archdochq/setup` downloads, which fails every download while it is replaced; a failed job would also leave a release without a bundle until re-run.
- **Telling a consumer that a release is published**, by a `repository_dispatch` event to a repository named by a variable, or by posting to an endpoint the consumer runs, as the release workflow does to publish this repository's specification. A consumer deploying from a repository would rebuild on each release with no schedule of its own. Lost because GitHub creates a `repository_dispatch` event only for a token with contents write on the receiving repository, so the release workflow would hold a credential able to change everything that repository holds only to send an event, where the credentials it holds for the Homebrew tap and the skills repository write what it publishes there; an endpoint would need a credential of its own as well. A consumer reads the release on a schedule or trigger of its own.
- **Writing the guides outside this repository and moving them in later.** Writing could begin before the generator exists. Lost because until the move, a guide can describe behaviour the code no longer has.
- **A title and an order in each guide's front matter.** Each guide would describe itself. Lost because orders collide, and moving one guide can mean renumbering others.
- **Leaving the order and titles of the guides to whatever displays them.** Lost because a guide a release adds would appear nowhere until each consumer changed.
- **Resolving the latest release when `install.sh` runs.** The repository's copy would work unrendered. Lost because the API is limited per address, which shared CI runners exhaust, and reading the release page's redirect differs between `curl` and `wget`, where a script attached to a release already names one.

## Backwards compatibility

Nothing a repository holds breaks. The archives, their names and `checksums.txt`'s name are unchanged. `checksums.txt` gains a line for the bundle and one for `install.sh`; `archdochq/setup` reads only the line naming its archive, and the Homebrew cask carries its own digests. A release now also stops before publishing when the guides, their contents or the reference fail the generator's checks.

## Changelog

- 2026-10-26: A release no longer sends a `repository_dispatch` event, and the `docs` job sending it is removed; a
  consumer reads the bundle and `install.sh` from the release.
