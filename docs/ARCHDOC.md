# ArchDoc

Tool specification. This describes what ArchDoc does in enough detail to build it. The process it enforces is described in `internal/template/PROCESS.md`, the copy shipped inside the binary and written by `init`, which is authoritative where the two disagree.

## Overview

ArchDoc is a Go CLI distributed as a static binary per platform via GitHub releases and `go install github.com/ollieread/archdoc/cmd/archdoc@latest`. It manages a spec repository: a directory containing `archdoc.json` and the four document directories.

The process is fixed. Configuration is limited to the five fields in `archdoc.json`.

## Repository layout

With the default `root` of `.`:

```
<repo>/
├── archdoc.json
├── README.md
├── PROCESS.md
├── INDEX.md              generated
├── LICENSE               optional
├── .github/workflows/archdoc-lint.yml
├── rfc/
├── adr/
├── spec/
│   └── glossary.md
└── ref/
```

With a `root` of `docs`, inside an existing code repository:

```
<repo>/
├── .github/workflows/archdoc-lint.yml
├── archdoc.json          root: "docs"
├── README.md             the code repository's own, untouched
└── docs/
    ├── README.md
    ├── PROCESS.md
    ├── INDEX.md          generated
    ├── rfc/
    ├── adr/
    ├── spec/
    │   └── glossary.md
    └── ref/
```

`root` in `archdoc.json` is relative to the file's location and defaults to `.`. All document directories are resolved under it, as are `README.md`, `PROCESS.md`, `INDEX.md` and `LICENSE`. This allows the spec to live in a subdirectory of a code repository without colliding with that repository's own README or licence. It must name that location or a directory below it: a `root` that is absolute, or that leads outside the directory holding `archdoc.json`, is a configuration error.

`.github/workflows/archdoc-lint.yml` is the exception: GitHub only runs workflows from the root of the git repository, so it is written there and never under `root`.

## Configuration

```json
{
  "name": "TheGamePanel",
  "branch": "main",
  "root": ".",
  "strict": true,
  "ref_stale_days": 180
}
```

- `name`: project name, used in README and INDEX headings.
- `branch`: the branch frozen documents are compared against.
- `root`: directory containing the four document directories, relative to `archdoc.json`, and at or below it.
- `strict`: when `true`, the transition graph is as in PROCESS.md. When `false`, `draft → accepted` and `draft → rejected` are additionally permitted.
- `ref_stale_days`: refs whose `verified` date is older than this produce a lint warning. `0` disables the check; a negative value is a config error.

Unknown keys are an error. Missing keys take the defaults above except `name`, which is required.

## Documents

### Discovery

Documents are discovered by directory. Every `*.md` file in `rfc/`, `adr/`, `ref/` and `spec/` is a document of that type. Files elsewhere are ignored. Subdirectories are ignored except under `spec/`, where they are permitted and pages are addressed by their path relative to `spec/`.

### Identifiers

Numbered types (`rfc`, `adr`, `ref`) use the identifier form `<PREFIX>-<NNNN>` where the prefix is the uppercased type and the number is four digits, zero-padded. Sequences are per type and start at 1. The next number for a type is one greater than the highest existing number in that directory, regardless of gaps. The sequence stops at 9999: the filename grammar requires exactly four digits, so a fifth would produce a document the tool could no longer read back. `new` refuses rather than writing one. Reaching the bound needs no 9999 documents, since a single hand-written `rfc/9999-old.md` is legal.

Filenames for numbered types are `<NNNN>-<slug>.md`. The slug is derived from the title: lowercased, non-alphanumerics collapsed to single hyphens, leading and trailing hyphens trimmed. The tool never renames files; if the title changes, the slug in the filename is left alone.

Spec pages are identified by their path relative to `spec/` without the `.md` extension. `spec/database.md` is `database`; `spec/http/routing.md` is `http/routing`.

### Front matter

Front matter is a YAML block delimited by `---` on the first line and a closing `---`. It is parsed with `gopkg.in/yaml.v3`, once into a `yaml.Node` whose mapping keys give presence and unknown-field detection, then decoded into the typed struct. A struct decode alone cannot distinguish `decided:` from an absent `decided`. Unknown fields are an error.

RFC and ADR:

| Field | Type | Rules |
|---|---|---|
| `id` | string | Required. Must equal the identifier derived from the type and filename number. |
| `title` | string | Required, non-empty. |
| `status` | enum | Required. One of `draft`, `proposed`, `accepted`, `rejected`, `withdrawn`. |
| `created` | date | Required. ISO 8601 date. |
| `decided` | date or empty | Required key. Must be set if and only if status is terminal. |
| `depends` | list of identifier | Required key, may be empty. RFC and ADR identifiers only. |
| `updates` | list of identifier | Required key, may be empty. Same type as this document, accepted only. |
| `obsoletes` | list of identifier | Required key, may be empty. Same type as this document, accepted only. |
| `backfilled` | date | Optional. The date this document was written to record a decision taken earlier. Implies a terminal status. |

`backfilled` is the only optional key, and every key added after it must be optional too. A new required key would make every frozen document in every existing repository fail L01, and L11 forbids the edit that would add it. The schema can therefore only ever grow optional fields.

Spec:

| Field | Type | Rules |
|---|---|---|
| `title` | string | Required, non-empty. |
| `includes` | list of identifier | Required key, may be empty. RFC and ADR identifiers only. |

Ref:

| Field | Type | Rules |
|---|---|---|
| `id` | string | Required. Must equal the derived identifier. |
| `title` | string | Required, non-empty. |
| `verified` | date | Required. ISO 8601 date. |

Terminal statuses are `accepted`, `rejected` and `withdrawn`.

### Body

The body is everything after the closing `---`. The tool parses headings (`#` through `######`) to locate sections. It does not otherwise interpret markdown except when resolving links.

A line inside a ``` or ~~~ fence is not a heading, and neither is a `#` indented four or more spaces. This holds wherever headings are read: section parsing, anchors and link scanning. Indented code blocks are not otherwise recognised.

Headings have anchors, computed as GitHub computes them: everything that is not a letter, number, underscore, hyphen or space is removed, the result is lowercased, each space becomes a hyphen, and a heading whose anchor is already taken within the same document gains a `-1`, `-2` suffix. Anchors are therefore assigned by walking a document's headings in order and are never derived from a heading in isolation. A heading is slugged from its rendered text, so `## The [RFC-0007](path) approach` anchors as `the-rfc-0007-approach`. The same algorithm serves glossary links and L17's anchor checks.

The first heading must be an H1 of the form `<id>: <title>` for numbered types, or `<title>` for spec pages. Lint checks this matches the front matter.

Section presence is checked by exact H2 text. Required sections by type:

- RFC: `Abstract`, `Motivation`, `Proposal`, `Alternatives considered`, `Backwards compatibility`, `Open questions`, `Changelog`, in that order. `Rejection rationale` is required when status is `rejected` and forbidden otherwise. `Sources` is required when `backfilled` is set. A document that requires either has a closing section, which must be its last H2: `Rejection rationale` when it is rejected, `Sources` otherwise. A document that requires neither may carry extra sections after the required ones.
- ADR: `Context`, `Decision`, `Alternatives`, `Consequences`, in that order. `Rejection rationale` and `Sources` as for RFC.
- Ref: `Sources` must be the last H2.
- Spec: no requirements, except the glossary (below).

A section is empty if it contains nothing but whitespace and HTML comments before the next heading of equal or higher level.

Text inside an HTML comment is not content anywhere: it is passed over when finding headings and sections, when resolving `[[...]]` links, and when suggesting them. This holds whether the comment closes on the line it opened on or several lines later, and a comment that is never closed runs to the end of the document. Text sharing a line with a comment, before its `<!--` or after its `-->`, is content.

Code wins over comments, and a comment wins over a fence it opened before. A `<!--` or `-->` inside an inline code span or a fenced code block is literal text and starts nothing, which is what lets a document describe the placeholders it ships with. A fence marker inside a comment is likewise literal. A comment and a fenced block are both blocks: whichever opens first holds the lines until it closes.

### Glossary

`spec/glossary.md` is a spec page with additional structure. Anything between the H1 and the first H2 is preamble and is ignored; new entries are inserted after it. From the first H2 onwards the body consists of H2 entries, one per term, each followed by exactly one paragraph. A glossary with no entries is valid. Entries must be unique (case-insensitive) and in ascending case-insensitive alphabetical order. A renamed term's paragraph may be followed by one or more lines of the form `Formerly *Old Term*.`, each on its own line, which do not count as further paragraphs; repeated renames accumulate lines rather than rewriting the existing one. `spec/glossary.md` need not exist, and L15 and glossary link resolution are skipped when it is absent.

## Relationships and derivation

Forward relationships are stored in front matter. The tool derives:

- `updated_by`: for each document, every document listing it in `updates`.
- `obsoleted_by`: for each document, every document listing it in `obsoletes`.
- `depended_on_by`: for each document, every document listing it in `depends`.
- `included_in`: for each RFC and ADR, every spec page listing it in `includes`.
- `implemented`: true for an accepted RFC or ADR with at least one `included_in`.
- `stale`: true for a spec page that includes a document that is effectively obsolete.

A document is **effectively obsolete** if any document in its `obsoleted_by` is accepted. Obsolescence by a draft, proposed, rejected or withdrawn document has no effect.

## Commands

All commands locate `archdoc.json` by walking up from the current directory and fail if none is found, except `init`. `-C, --chdir <dir>` is a persistent flag that runs as if archdoc had been started in that directory, applied before anything reads the working directory. A specification repository is commonly a sibling of the code it documents rather than an ancestor of it, so walking up never reaches it; without the flag every invocation from the code repository would have to be wrapped in a `cd`. All commands operate relative to `root`. Exit code is 0 on success, 1 on a usage or validation error, 2 when lint or check finds errors.

**Interaction.** When stdin is a TTY, a command prompts for every value it was not given: required arguments, and optional flags whose value is written to a file. Optional flags that select a mode of operation rather than supply a value (`--json`, `--check`, `--suggest`, `--apply`, `--strict-warnings`, `--no-interaction`) are never prompted for. A flag supplied on the command line is not prompted for. The prompts for one invocation are presented as a single form, required arguments first, each optional pre-filled with its default; accepting an optional prompt unchanged takes the default or an empty selection. When stdin is not a TTY, or `--no-interaction` is given, defaults apply and a missing required argument is a usage error. `--no-interaction` is a persistent flag, available on every command.

Prompts offer a filtered choice wherever one exists: `new` prompts for type then title; the transition commands prompt with a select of documents eligible for that transition, showing identifier, title and current status; `term add` prompts for term and definition, then a multi-select of accepted documents for `--from`; `init` prompts for each of its configuration values in turn.

Output is plain text to stdout, one finding per line in the form `<path>:<line>: <severity>: <message>` where a line is known, and `<path>: <severity>: <message>` otherwise. `<path>` is relative to `root`, so output does not depend on the working directory. `--json` on `lint`, `index --check` and `link` emits the same findings as a bare JSON array of objects with `path`, `line`, `severity`, `message` and `rule`; `line` is omitted where it is unknown, and `rule` is the rule code, or `index` for the index check.

### `archdoc init`

Scaffolds a repository in the current directory.

Options:

- `--name=<string>`: project name. Defaults to the directory name with a trailing `-spec` removed.
- `--branch=<string>`: defaults to the current git branch if the directory is inside a git repository, otherwise `main`.
- `--root=<path>`: defaults to `.`.
- `--strict` / `--no-strict`: defaults to strict.
- `--ref-stale-days=<int>`: defaults to 180.
- `--license=<none|mit>`: defaults to `none`.
- `--no-interaction`: never prompt; fail if a required value has no default.

Writes `archdoc.json` in the current directory; `README.md`, `PROCESS.md`, `INDEX.md`, `rfc/.gitkeep`, `adr/.gitkeep`, `ref/.gitkeep`, `spec/glossary.md`, and `LICENSE` if requested, under `root`; and `.github/workflows/archdoc-lint.yml` at the git repository root, or in the current directory when there is no repository. Prints every path written. Refuses to run if `archdoc.json` already exists, and refuses to overwrite any other existing file. Does not initialise git and does not commit.

`README.md` is short: the project name, one sentence per document type, and a link to PROCESS.md.

`archdoc-lint.yml` runs on push and pull request, downloads the `archdoc` binary at the version that ran `init`, and runs `archdoc lint` then `archdoc index --check`. Its `working-directory` is the path from the git repository root to the directory holding `archdoc.json`, omitted when they are the same, because commands locate `archdoc.json` by walking up.

### `archdoc new <rfc|adr|ref> <title>`

Creates the next-numbered document of the type from its embedded template with `ID`, `Title` and `Date` (today) substituted. Prints the path. Refuses if the derived filename already exists, or if the title is empty or slugs to nothing.

Options:

- `--backfill`: create the document already terminal, recording a decision taken before it was written. RFCs and ADRs only; a ref has no status. Sets `backfilled` to today, pre-fills `Alternatives considered` and `Backwards compatibility` with `Not recorded.` rather than a prompt, and adds a `Sources` section.
- `--status=<accepted|rejected|withdrawn>`: with `--backfill`, the terminal status to create. Defaults to `accepted`.
- `--created=<date>`, `--decided=<date>`: with `--backfill`, the historical dates. `--decided` defaults to `--created`.

Backfilling is a mode of creation, not a lifecycle path: a backfilled document was never proposed, so it uses no transition and the transition graph below is unaffected.

### Transitions

Strict mode (default):

| From | To |
|---|---|
| `draft` | `proposed`, `withdrawn` |
| `proposed` | `accepted`, `rejected`, `withdrawn` |
| `accepted` | none |
| `rejected` | none |
| `withdrawn` | none |

Non-strict mode adds `draft → accepted` and `draft → rejected`. No other transition exists in either mode. Terminal statuses have no outgoing transitions. `proposed → draft` is never permitted.

### `archdoc propose <id>`, `accept <id>`, `reject <id>`, `withdraw <id>`

Transitions a document. Each:

1. Loads the document and fails if the identifier is unknown or the type is not RFC or ADR.
2. Checks the transition is permitted from the current status under the `strict` setting. Fails with a message naming the permitted transitions from the current status.
3. For `accept` and `reject`: fails if any required section is empty, other than `Open questions` and `Changelog`; fails if any unresolved `[[...]]` remains. For `accept` additionally: fails if the document has an `Open questions` section and it is non-empty. ADRs have no such section, so the check does not apply to them unless one is present as an extra section. `withdraw` performs no content checks.
4. Rewrites `status` and, for terminal transitions, `decided` to today. Front matter is rewritten field-by-field preserving order and comments; the body is not touched except as below.
5. For `reject`: appends `## Rejection rationale` to the end of the body with an HTML comment prompting for the rationale. The step 3 checks run before this append. Until the rationale replaces that comment L14 reports the section as empty, which is the intended prompt to finish the document before committing it.
6. Prints the new status.

Does not commit.

### `archdoc lint`

Runs every rule below over every document and prints findings. Exit code 2 if any error, 0 otherwise. Warnings do not affect the exit code unless `--strict-warnings` is given, in which case a warning alone also exits 2.

Rules. Severity is `error` unless stated. A finding that no permitted edit to the named document could clear is a warning rather than an error, because L11 forbids that edit. This softening applies to the rules whose subject is the document's own body, which are L14, L16 and L17. It does not apply to a fault in a relationship between documents, such as L04's, which is cleared by editing the other document rather than the one named, nor to L11 itself.

- **L01** Front matter parses and matches the schema for the type. Unknown fields, missing required keys, wrong types.
- **L02** `id` equals the identifier derived from type and filename. Filename matches `<NNNN>-<slug>.md`.
- **L03** No identifier appears on more than one document.
- **L04** Every identifier in `depends`, `updates`, `obsoletes`, `includes` resolves to an existing document of a permitted type.
- **L05** `updates` and `obsoletes` reference only accepted documents of the same type. No document updates or obsoletes itself. No document appears in both lists of the same document.
- **L06** `includes` references only accepted documents.
- **L07** `decided` is set if and only if status is terminal. `decided` is not before `created`. If `backfilled` is set, the status is terminal and `backfilled` is not before `decided`.
- **L08** Required sections are present, with exact titles, in the listed relative order. Other H2 sections are permitted between or after them. `Rejection rationale` present if and only if rejected. Where the per-type list above gives a closing section, it must be the document's last H2; for a backfilled document that is `Sources`, so that the reconstruction has something it can be checked against, and for a rejected one it is `Rejection rationale`, which PROCESS.md makes final. L14 holds each to the same standard as every other required section.
- **L09** An accepted document has no non-empty `Open questions` section. Required on RFCs by L08; absent from ADRs.
- **L10** The H1 matches the front matter.
- **L11** Frozen documents are unchanged. For every document whose status on `branch` is terminal, the working tree file is unchanged from the file at `branch`. Whether it has changed is decided by git rather than by comparing bytes, because git applies checkout filters between the object it stores and the file on disk: under end-of-line normalisation, which is the default on Windows, every frozen document differs from its stored blob while the tree is clean. If the file does not exist on `branch` it is new and the rule does not apply. A document that is terminal on `branch` and absent from the working tree is an error; a rename is a deletion and a new file. If the repository has no commits yet, nothing can be frozen and the rule is skipped with a warning. If `branch` does not exist in a repository that does have commits, the configured name is wrong and the rule reports an error naming it, rather than passing every document silently. If not inside a git repository, this rule is skipped with a warning.
- **L12** A spec page is not stale: it includes no document that is effectively obsolete. Warning.
- **L13** A ref's `verified` date is not older than `ref_stale_days`. Warning.
- **L14** No required section is empty, except `Open questions` and `Changelog`. Warning for `proposed`; error for a document that is terminal in the working tree but not yet terminal on `branch`, which is the last point at which it can be fixed; warning once it is frozen. Not applied to `withdrawn`.
- **L15** Glossary structure: after the preamble, H2 entries only, unique, alphabetical, one paragraph each. No entries is valid.
- **L16** No unresolved `[[...]]` wiki link anywhere. Error in editable documents; warning in frozen ones.
- **L17** Every relative markdown link resolves to an existing file, and if it has an anchor, to an existing heading. Error in editable documents; warning in frozen ones.
- **L18** Every `depends` entry references a document that is accepted, or that shares the referencing document's non-terminal status. Warning. (A proposed RFC may depend on another proposed RFC; an accepted RFC depending on a draft is suspicious.)

### `archdoc index [--check]`

Generates `INDEX.md` under `root` and prints the path written. With `--check`, generates to memory and exits 2 if it differs from the file on disk, printing a unified diff and nothing at all when the index is current. With `--json` the difference is reported as a single finding against `INDEX.md` and the diff is not printed.

Format:

```markdown
# <name> index

Generated by ArchDoc. Do not edit.

## RFCs

| ID | Title | Status | Decided | Updates | Updated by | Obsoletes | Obsoleted by | Implemented in |
|---|---|---|---|---|---|---|---|---|

## ADRs

(same columns)

## Spec

| Page | Includes | Stale |
|---|---|---|

## Refs

| ID | Title | Verified |
|---|---|---|
```

All four sections are always present with their header rows, even when empty, so the file does not change shape as the repository grows. Rows are sorted by identifier for numbered types and by path for spec pages. Every identifier and page in every cell is a relative markdown link. Empty cells are empty. A status carries its qualifiers in parentheses, comma-separated in the order backfilled, obsolete: `accepted (backfilled)`, `accepted (obsolete)`, `accepted (backfilled, obsolete)`. `Stale` is `yes` or empty.

### `archdoc link [--suggest] [--apply]`

Resolves wiki links and, optionally, suggests new ones.

Default behaviour: for every editable document (non-terminal RFC and ADR, every spec page, every ref), replace each `[[X]]` with a markdown link:

- If `X` is an identifier: `[X: <title>](<relative path>)`.
- If `X` matches a glossary term (case-insensitive): `[X](<relative path to glossary>#<anchor>)`, preserving the case as written.
- If `X` matches both, the glossary wins.
- If `X` matches neither: error, file unchanged.

Frozen documents containing `[[...]]` produce a warning naming the document, matching L16; they are never modified. An unresolvable `[[X]]` in an editable document stays an error, because it is both the author's mistake and fixable. Findings from `link` follow `lint`'s exit codes; exit 1 stays reserved for usage errors.

With `--suggest`: additionally scan editable documents for bare identifiers and glossary terms not already inside a link or code span, and propose linking the first occurrence of each distinct target per document. Excludes the document's own identifier, every heading, front matter, code blocks, bare URLs, link reference definitions and the glossary page's own entries. A heading's text is a section's identity to L08, L09, L14 and the freeze gate, so neither `link` nor `link --suggest` rewrites one; an unresolved wiki link in a heading is reported by L16 and corrected by hand.

A suggestion wraps the matched text exactly as written and never alters it: `RFC-0007` becomes `[RFC-0007](<relative path>)`, not `[RFC-0007: <title>](<relative path>)`. Expanding `[[X]]` supplies the title because the author wrote a placeholder expecting substitution; a bare identifier in prose is finished text. Identifiers match the canonical uppercase form on word boundaries. Glossary terms match whole, case-insensitively, on word boundaries, longest match first, with no stemming and no plural handling.

Suggestion handling:

- If stdin is a TTY and `--apply` is not given: interactive. For each suggestion, show the file, line, and the line with the match highlighted, and prompt `[y]es [n]o [a]ll in file [s]kip file [q]uit`. `q` writes accepted changes so far and exits.
- If stdin is not a TTY and `--apply` is not given: print suggestions, exit 2 if any.
- With `--apply`: apply all without prompting.

### `archdoc term <add|rename|remove|list|show>`

Manages `spec/glossary.md`.

- `add <term> <definition> [--from <id>...]`: insert alphabetically, creating `spec/glossary.md` from the template if it does not exist. Fails on duplicate. Each `--from` identifier is appended to the page's `includes` if not present.
- `rename <old> <new> [--from <id>...]`: retitle the entry and add `Formerly *<old>*.` after its paragraph. Re-sorts. Does not rewrite links elsewhere; `lint` L17 reports them, and they must be corrected by hand. `link` cannot repair them: it expands `[[...]]` and never edits an existing markdown link, and a reference written as `[[<old name>]]` no longer resolves either, because a recorded former name is not a term.
- `remove <term> [--from <id>...]`: delete the entry.
- `list`: print terms, one per line.
- `show <term>`: print the entry.

## Templates

Embedded under `internal/template`: `rfc.md`, `adr.md`, `ref.md`, `glossary.md`, `PROCESS.md`, `README.md`, `archdoc-lint.yml`, `LICENSE.mit`. Substitution uses `text/template` with `.ID`, `.Title`, `.Date`, `.Name`, `.Version`, `.Status`, `.Decided`, `.Backfilled`, `.Year` and `.WorkingDirectory` as applicable.

One function is registered: `yaml`, which encodes a value as a YAML scalar, quoting and escaping only where the encoding requires it. Front matter uses `title: {{ yaml .Title }}`, so a title containing a colon or any other YAML-significant character produces a document that parses. The H1 uses `.Title` raw and is read back by splitting on the first `": "`.

## Package layout

```
archdoc/
├── cmd/archdoc/         cobra commands; no logic beyond argument handling
├── internal/
│   ├── config/          archdoc.json loading and defaults
│   ├── repo/            discovery, parsing, identifiers, derivation
│   ├── lint/            rules, each a function returning findings
│   ├── lifecycle/       transition graph and file rewriting
│   ├── index/           INDEX.md generation
│   ├── link/            wiki link resolution and suggestion
│   ├── glossary/        glossary parsing and editing
│   ├── git/             interface with a shell-out implementation
│   ├── repotest/        throwaway repositories for tests
│   └── template/        embedded files
```

`cmd/archdoc` is the only package that imports cobra. Every other package is usable without a terminal, so a later desktop or TUI frontend calls them directly, and nothing under `internal/` calls `os.Exit`.

`internal/git` is an interface with a shell-out implementation, exposing `FileAt`, `FilesAt`, `Changed`, `HasCommits`, `BranchExists`, `ListFiles`, `CurrentBranch` and `RepoRoot`, and nothing else until something needs more. It returns a sentinel error when the directory is not inside a git repository, so a caller can tell that from a file being absent on `branch`. The batch forms exist because a subprocess per document was the whole of lint's running time on a repository of any size: `FilesAt` reads many blobs through one `cat-file --batch` and `Changed` asks one `diff` which paths differ. Both use NUL-separated requests and responses, because git permits a newline in a path and a line-oriented protocol pairs those wrongly.

`internal/repotest` builds throwaway repositories for tests. It imports `testing`, as `httptest` and `iotest` do, and sits under `internal/` so it never reaches a binary.

## Dependencies

- `github.com/spf13/cobra`
- `gopkg.in/yaml.v3`
- `github.com/charmbracelet/huh` for interactive prompts
- `github.com/charmbracelet/x/term` to detect a terminal
- `github.com/goreleaser/goreleaser` at build time only

Nothing else. Markdown is handled with the standard library; the tool only needs headings, comments, fenced code blocks and links, and a full parser would be a liability when rewriting files in place.

## Release

Tagged versions build with goreleaser for linux, darwin and windows on amd64 and arm64. `archdoc --version` prints the tag alone.

The version resolves in order: the ldflag goreleaser sets at build time; failing that the module version from `runtime/debug.ReadBuildInfo`, which is populated for `go install ...@version` and for `@latest`; failing that `dev`. `init` records the resolved version in the generated `archdoc-lint.yml` so a repository pins the tool that scaffolded it. When it resolves to `dev`, `init` writes `latest` instead and warns that the workflow is unpinned: a workflow that works unpinned is better than one pinned to a release that does not exist.

The workflow downloads the release asset and verifies it against the published checksums file. Asset names are fixed by `.goreleaser.yaml`; the two must agree. The agreement is tested: the archive name, the archive format, the checksum filename and the binary's name are each resolved from `.goreleaser.yaml` and compared against what the workflow builds, so a rename of one side alone fails the suite and a coordinated rename of both does not.
