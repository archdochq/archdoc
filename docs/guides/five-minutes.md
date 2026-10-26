# Five minutes with ArchDoc

This walk-through creates a specification repository, writes one RFC, takes it to `accepted`, and shows what happens when you edit it afterwards. You need `archdoc` and git on `PATH`; [Installing ArchDoc](installing.md) covers the first.

Everything here happens on `main` to keep it short. In a shared repository each document is written on a branch and offered as a pull request, which [Contributing by pull request](pull-requests.md) describes.

## Create the repository

```sh
mkdir myproject-spec && cd myproject-spec && git init -b main
archdoc init
```

On a terminal, `init` asks for each setting in turn, each pre-filled with its default. Accept them all, or pass `--no-interaction` to take the defaults without asking:

| Setting | Flag | Default |
|---|---|---|
| Project name | `--name` | The directory name with a trailing `-spec` removed, so `myproject` here |
| Branch | `--branch` | The current git branch, or `main` |
| Document root | `--root` | `.` |
| Strict lifecycle | `--strict` / `--no-strict` | strict |
| Days before a ref is stale | `--ref-stale-days` | `180` |
| Licence | `--license` | `none`; `mit` writes an MIT `LICENSE` |
| Coding agent guides | `--agents` | off |

It prints every file it writes:

```text
README.md
PROCESS.md
rfc/.gitkeep
adr/.gitkeep
ref/.gitkeep
term/.gitkeep
.github/workflows/archdoc-lint.yml
archdoc.json
INDEX.md
GLOSSARY.md
```

- `archdoc.json` holds the settings. Every other command finds the repository by walking up from the current directory to this file.
- `PROCESS.md` describes the process ArchDoc enforces: the document types, the lifecycle, and the rules. Read it once.
- `INDEX.md` and `GLOSSARY.md` are generated. Do not edit them by hand.
- `rfc/`, `adr/`, `ref/` and `term/` hold RFCs, ADRs, refs and glossary terms. Spec pages go in `spec/`, which you create when you write the first one.
- `.github/workflows/archdoc-lint.yml` runs the checks on every push and pull request, pinned to the version of `archdoc` that wrote it. [Checking in CI](ci.md) covers it.

`init` refuses to run where `archdoc.json` already exists, and does not commit anything.

## Commit the scaffold

```sh
git add -A
git commit -m "Scaffold the specification"
```

Commit before writing any documents. The frozen-document rule compares your working tree with the configured branch, and with no commits there is nothing to compare against. Before this commit, `archdoc lint` says so:

```text
.: warning: skipped: the repository has no commits yet, so no document can be frozen
```

## Lint

```sh
archdoc lint
```

A repository with nothing wrong prints nothing and exits 0. Each finding is one line, `<path>:<line>: <severity>: <message>`, without `:<line>` where the finding has no line. Lint exits 2 when it reports an error; warnings alone exit 0 unless you pass `--strict-warnings`.

## Write an RFC

```sh
archdoc new rfc "Retry policy"
```

```text
rfc/0001-retry-policy.md
```

The RFC takes the next free number. It starts as a draft, with its front matter filled in and a comment in each section saying what belongs there:

```yaml
---
id: RFC-0001
title: Retry policy
status: draft
created: 2026-10-26
decided:
depends: []
updates: []
obsoletes: []
---
```

`created` is today's date. Below the front matter are the H1 and the sections: `Abstract`, `Motivation`, `Proposal`, `Alternatives considered` and `Backwards compatibility`, which are required, and `Open questions` and `Changelog`, which are optional. Write each required section, replacing its comment. Keep the section titles exactly as they are: lint checks them. Leave `Open questions` empty, or delete it, once nothing is open.

A draft may have empty sections. Lint starts reporting them once the document is proposed.

## Propose it

```sh
archdoc propose RFC-0001
```

```text
RFC-0001 is now proposed
```

`propose` changes `status` and nothing else. If you propose before writing the sections, lint warns about each empty one:

```text
rfc/0001-retry-policy.md:14: warning: section "Abstract" is empty
```

Proposing is when the document is offered for review. Regenerate the index and commit:

```sh
archdoc index
git add -A
git commit -m "Propose RFC-0001"
```

## Regenerate the index

`archdoc index` rewrites `INDEX.md`, which lists every document with its status and every relationship ArchDoc derives between them. Run it after adding a document or changing one's status, title or relationships, and commit the result with the change.

`archdoc index --check` writes nothing. It prints the difference it would make and exits 2 if `INDEX.md` is out of date, which is what the scaffolded workflow runs. Had you run `archdoc index` while RFC-0001 was a draft, and proposed it without running it again, it would print:

```text
--- INDEX.md
+++ INDEX.md (generated)
@@ -6,7 +6,7 @@
 
 | ID | Title | Status | Decided | Updates | Updated by | Obsoletes | Obsoleted by | Implemented in |
 |---|---|---|---|---|---|---|---|---|
-| [RFC-0001](rfc/0001-retry-policy.md) | Retry policy | draft |  |  |  |  |  |  |
+| [RFC-0001](rfc/0001-retry-policy.md) | Retry policy | proposed |  |  |  |  |  |  |
 
 ## ADRs
 
```

## Accept it

Once the proposal is on the configured branch, accept it:

```sh
archdoc accept RFC-0001
```

```text
RFC-0001 is now accepted
```

`accept` sets `status` to `accepted` and `decided` to today. It refuses while `Open questions` holds anything, and while lint reports an error about the document as it would be once accepted. An accepted RFC with empty required sections is an error, not a warning:

```text
rfc/0001-retry-policy.md:14: error: section "Abstract" is empty
archdoc: RFC-0001 cannot become accepted while lint reports errors about it
```

In a strict repository, the default, a draft cannot be accepted directly:

```text
archdoc: draft cannot become accepted; from draft a document may become proposed, withdrawn
```

Regenerate the index and commit the verdict as one commit:

```sh
archdoc index
git add -A
git commit -m "Accept RFC-0001"
```

`reject` and `withdraw` work the same way. `reject` also appends a `Rejection rationale` section for you to fill in.

## See the freeze

`accepted`, `rejected` and `withdrawn` are terminal. A document with a terminal status is frozen once it is committed on the configured branch: it is never changed again. Edit the RFC you just accepted, change a word anywhere in it, and run lint:

```sh
archdoc lint
```

```text
rfc/0001-retry-policy.md: error: changed since it was frozen on main: a terminal document is never modified
```

Lint exits 2. `archdoc lint --json` shows the rule behind the finding:

```json
[
  {
    "path": "rfc/0001-retry-policy.md",
    "severity": "error",
    "message": "changed since it was frozen on main: a terminal document is never modified",
    "rule": "L11"
  }
]
```

Undo the edit with `git checkout -- rfc/0001-retry-policy.md` and lint is clean again. The transition commands respect the freeze too:

```sh
archdoc withdraw RFC-0001
```

```text
archdoc: accepted is terminal, so it cannot become withdrawn: write a new document that updates or obsoletes it
```

To change what an accepted RFC says, write a new RFC and list the old one in its `updates` or `obsoletes`. Once the design is built, describe how it works on a spec page under `spec/` and list the RFC in that page's `includes`. [Concepts](concepts.md) explains the document types and how they relate.
