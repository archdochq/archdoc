# ArchDoc

A CLI that manages a specification repository: a directory of Markdown documents
with YAML front matter, a fixed lifecycle, and cross-references the tool derives
rather than asks you to maintain.

There are four kinds of document. An **RFC** proposes how something should work
and gets a spec page once it is built. An **ADR** records a decision that
constrains designs without being a thing the spec describes. The **spec**
describes how the project works right now and is always current. A **ref** holds
research and background, and is never normative.

RFCs and ADRs are frozen once they reach a terminal status. To change what an
accepted document says you write a new one that updates or obsoletes it, and the
tool enforces that: eighteen lint rules, a generated index of every derived
relationship, and a CI workflow it scaffolds for you.

## Installing

With Homebrew, on macOS or Linux:

```
brew install archdochq/tap/archdoc
```

Or download a binary for your platform from the
[releases page](https://github.com/archdochq/archdoc/releases), or with Go:

```
go install github.com/archdochq/archdoc/cmd/archdoc@latest
```

`go install` puts it in `$(go env GOPATH)/bin`, which is often not on `PATH`. If
the command is not found afterwards, that is why.

ArchDoc is pre-1.0 and currently alpha. The document format and the process are
settled; the command surface may still move.

## Getting started

```
mkdir myproject-spec && cd myproject-spec && git init
archdoc init
archdoc new rfc "The first design"
archdoc lint
```

`archdoc init` scaffolds the repository, writes `PROCESS.md` describing the
process it enforces, and generates a GitHub workflow that runs `archdoc lint`
and `archdoc index --check` on every push.

## Commands

| | |
| --- | --- |
| `init` | Scaffold a repository. `--agents` adds guidance for coding agents |
| `new rfc\|adr\|ref "Title"` | Create the next-numbered document, in draft |
| `propose`, `accept`, `reject`, `withdraw` | Move a document through its lifecycle |
| `lint` | Check every rule |
| `index [--check]` | Regenerate `INDEX.md`, or verify it is current |
| `link [--suggest]` | Resolve `[[...]]` links, or offer new ones |
| `term add\|rename\|remove\|list\|show` | Maintain the glossary |
| `renumber <id\|path> [new-id]` | Change a document's number, following every reference |
| `export [--out <dir>]` | Write the repository as JSON |
| `agents` | Install or refresh the guides for coding agents |
| `update` | Refresh what ArchDoc generates, after upgrading it |

`-C <dir>` runs against another directory, which a specification repository
sitting beside the code it documents needs.

## Using the data elsewhere

`archdoc export` writes the repository as JSON: every document with its front
matter, its parsed sections, its resolved links, and the relationships ArchDoc
derives. `--schema` prints the JSON Schema the output conforms to.

That exists so a site rendering your specification does not reimplement the
front matter schema, the identifier rules or the reverse-relationship graph.
Bodies are raw Markdown; ArchDoc has no renderer and is not acquiring one.

## Working with coding agents

`archdoc init --agents` writes `AGENTS.md` and a set of guides into the
repository, covering classification, authoring, backfilling, lint triage and
contributing by pull request. `archdoc agents` installs or refreshes them later.

The same guides are packaged as skills under `plugin/`, for hosts that discover
them by name:

```
/plugin marketplace add archdochq/archdoc
/plugin install archdoc@archdoc
```

One source, three deliveries, compared byte for byte by a test.

## Documents

- `docs/ARCHDOC.md` specifies what the tool does.
- `internal/template/PROCESS.md` specifies the process it enforces, and is
  authoritative where the two disagree. It is also the copy shipped inside the
  binary and written by `archdoc init`.
- `docs/DECISIONS.md` records why the tool works the way it does, grouped by
  subject, including the alternatives that were tried and rejected.
- `docs/TESTING.md` covers how the suite is built and how to run it.
- `docs/EXPORT.md` describes the JSON `archdoc export` produces.

## Licence

GNU AGPLv3. The full text is in `LICENSE`.

Use it however you like, including inside a company and including commercially.
The obligation only lands if you redistribute a modified archdoc, or run one as
a network service: then you must publish your source under the same licence.
Running it on your own documents creates no obligation at all.

`archdoc init --license=mit` scaffolds an MIT licence for the spec repository
you are creating. That covers your documents and is unrelated to ArchDoc's own
licence.
