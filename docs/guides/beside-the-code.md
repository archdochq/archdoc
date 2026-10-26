# A spec repository beside the code

A specification repository records how a piece of software is meant to work and how it works now. It can live in a directory of the code repository, or in a repository of its own. This guide covers setting up each, running commands against it, and how its documents relate to the code.

## Choosing an arrangement

In a directory of the code repository, a pull request can change the code and the documents together. The documents share the code's branches and history, and the configured branch is the code's main branch.

In a separate repository, the documents have their own history, their own pull requests and their own reviewers. The code repository holds the evidence (commits, pull requests, issues) and the specification repository holds the record, and you run `archdoc` across from one to the other.

## In a directory of the code repository

### Scaffolding

Create the directory and run `archdoc init` inside it:

```sh
cd myproject
mkdir docs && cd docs
archdoc init --name MyProject
```

`init` finds the root of the git repository and writes `archdoc.json` there, with `root` set to the path down to the directory you ran it in. The documents land in that directory:

```text
myproject/
├── .github/workflows/archdoc-lint.yml
├── archdoc.json          root: "docs"
├── README.md             the code repository's own
└── docs/
    ├── README.md
    ├── PROCESS.md
    ├── INDEX.md
    ├── GLOSSARY.md
    ├── rfc/
    ├── adr/
    ├── ref/
    └── term/
```

Running `archdoc init --root docs` from the root of the repository puts the documents in the same place, though the name it offers is then the repository's directory name. `--root` is joined to the path from the repository root to where you ran `init`, so both forms put the documents in `docs/`.

On a terminal, `init` asks for each setting no flag gives, offering a default. Pass `--name` to set the name without being asked. Its default is the name of the directory you ran `init` in, with a trailing `-spec` removed, which here would be `docs`. The name appears in the headings of `README.md` and `INDEX.md`.

`init` refuses if `archdoc.json` already exists, or if any file it would write is already there. It writes the workflow to `.github/workflows/archdoc-lint.yml` at the repository root, the only place GitHub reads workflows from. It does not commit: review what it wrote, then commit it.

There is no `spec/` directory until you write the first spec page, and no `group/` directory unless you add one. Both are created by hand.

### Running commands

Every command except `init` finds `archdoc.json` by walking up from the directory it is run in. With `archdoc.json` at the repository root, that is any directory in the code repository:

```sh
archdoc lint
archdoc new rfc "Retry policy"
```

Paths in the output are relative to `root`, so a finding reads `rfc/0001-retry-policy.md:4: error: ...` rather than starting with `docs/`.

### Repositories scaffolded with `archdoc.json` in the subdirectory

Before `init` wrote `archdoc.json` at the repository root, it wrote it in the directory it was run in, with `root` of `.`. A repository scaffolded that way works unchanged. Commands find the configuration by walking up, so they work from inside that directory and below it, and from anywhere else with `-C`:

```sh
archdoc -C spec lint
```

The workflow for such a repository carries the path to that directory, as the `path` input of `archdochq/lint` and as `working-directory` on the steps that run `archdoc`. `archdoc update` writes both when it regenerates the workflow. The pre-commit hooks need `args: [-C, spec]`, as described in [Checking in CI](ci.md#pre-commit).

## In a separate repository

Create the repository, scaffold it and commit the scaffold before writing any document:

```sh
mkdir myproject-spec && cd myproject-spec
git init
archdoc init
git add -A && git commit -m "Scaffold the specification"
```

Here the default name is `myproject`, from the directory name with `-spec` removed. `archdoc.json` sits at the root with `root` of `.`.

Until the repository has a commit, the frozen-document check is skipped with a warning:

```text
.: warning: skipped: the repository has no commits yet, so no document can be frozen
```

### Running commands from the code repository

A common layout keeps the two repositories side by side, such as `myproject/` next to `myproject-spec/`. From inside the code repository, walking up never reaches the specification repository, and a command fails with `no archdoc.json found`. Use `-C` (or `--chdir`) to run it as if it had been started there:

```sh
archdoc -C ../myproject-spec lint
archdoc -C ../myproject-spec new adr "Store timestamps in UTC"
```

`-C` is accepted before or after the command name, and is applied before anything reads the working directory.

## The branch, and when a document freezes

`archdoc.json` records the branch that frozen documents are compared against:

```json
{
  "name": "MyProject",
  "branch": "main",
  "root": "docs",
  "strict": true,
  "ref_stale_days": 180
}
```

`init` takes the branch from `--branch`, or failing that the branch currently checked out, or `main` where there is none. In a code repository that is usually the code's main branch. If the code repository merges into a different branch, pass `--branch`, or edit `branch` afterwards and run `archdoc update` to regenerate the workflow, which names the branch.

Freezing follows that branch. An RFC or ADR is frozen once its terminal status (`accepted`, `rejected` or `withdrawn`) is on the configured branch, and from then on `archdoc lint` reports an error for any change to it, or for its deletion:

```text
rfc/0001-retry-policy.md: error: changed since it was frozen on main: a terminal document is never modified
```

The comparison reads the branch from git: the local branch, or `origin/<branch>` where there is no local branch, as in a CI checkout of a pull request. A document that is terminal only on another branch is not frozen yet. Where the configured branch does not exist in a repository that has commits, lint reports an error naming it.

## How the documents relate to the code

### RFCs come before the code

An RFC proposes how something should work, before it is built. Write it with `archdoc new rfc "Title"`, propose it with `archdoc propose`, and once its acceptance is on the configured branch it is frozen. The code is written from it. An accepted RFC is not marked as built: whether it is implemented is recorded by a spec page, below.

An ADR records a decision that constrains designs without being a thing the spec describes, such as a choice of storage format, with the alternatives that were turned down. It follows the same lifecycle and freezes in the same way.

To change what an accepted RFC or ADR says, write a new one that lists it under `updates` or `obsoletes`.

### Spec pages describe the code as it is

A spec page describes how the software works now, in the present tense, and is edited whenever that behaviour changes. Spec pages have no status and are never frozen. `archdoc new` does not create them: write the file under `spec/` by hand, with a `title`, an `includes` list and an H1 matching the title:

```markdown
---
title: Retries
includes: [RFC-0001]
---

# Retries

Requests are retried up to five times, with exponential backoff.
```

`includes` lists the accepted RFCs and ADRs the page currently reflects, and is how implementation is recorded. An accepted RFC that no spec page includes is not yet implemented. `INDEX.md` shows, for each RFC and ADR, the spec pages that include it, in its `Implemented in` column. When an RFC a page includes is later obsoleted, lint warns that the page is stale until the page is updated and its `includes` changed.

`PROCESS.md` asks that a spec page be updated in a commit that references, by URL, the commit or pull request in the code repository that changed the behaviour, and that `includes` be updated in the same commit.

The spec describes and does not argue. Reasons belong in the RFCs and ADRs a page includes.

### Refs hold background

A ref holds research, comparisons and other background, or the record of an exchange such as a review or a meeting. It is never normative and may be edited at any time. Create one with `archdoc new ref "Title"`, and with `--as-of <date>` for the record of an exchange. A ref carries a `verified` date, and lint warns when that date is older than `ref_stale_days` (180 by default), except on a ref carrying `as_of`. Other documents cite a ref by its identifier from their body.

## Checking it

Whichever arrangement you choose, the checks are the same: `archdoc lint`, `archdoc index --check` and `archdoc glossary --check`, and `archdoc lint --merge` on a pull request into the configured branch. [Checking in CI](ci.md) covers running them on GitHub, on GitLab and before each commit.
