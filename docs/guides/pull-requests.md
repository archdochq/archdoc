# Contributing by pull request

In an ArchDoc repository, a document is written on a branch and offered as a pull request into the configured branch, the `branch` set in `archdoc.json`. Discussion of the document happens on its pull request. This holds for a maintainer's own documents as well as for a contribution to a repository you do not maintain.

The author of a document writes it, proposes it, and withdraws it if it is no longer wanted. Accepting or rejecting it is a maintainer's. ArchDoc does not check who made a change: who may merge a pull request that gives a verdict is decided by the code host, through branch protection and code owners.

## Before you start

You need `archdoc` on your `PATH`; see [Installing ArchDoc](installing.md). Decide which type of document you are writing. An RFC proposes how something should work and will have a spec page once built; an ADR records a decision that constrains designs without being something the spec has a page for. [Concepts](concepts.md) covers the types.

## Draft on a branch

Fork the repository if you need to, clone it, and create a branch. Then create the document with the tool:

```sh
archdoc new rfc "Retry policy for scheduled jobs"
```

`archdoc new` prints the path it wrote, such as `rfc/0007-retry-policy-for-scheduled-jobs.md`, with the next free number, the front matter, the heading and every section from the template, in `draft`. Do not create the file by hand: the file name and `id` carry the document's identity, and lint rejects one that does not match its type and file name.

Fill every required section. While the document is a draft it may change freely.

## Propose before review

When the document is ready for comment, propose it:

```sh
archdoc propose RFC-0007
```

This sets `status: proposed` and prints `RFC-0007 is now proposed`. It does not commit; commit the change yourself.

A draft cannot merge. On a pull request into the configured branch, CI runs `archdoc lint --merge`, and rule M01 fails a document that arrives as a draft. Propose it before you ask for review.

## Before you push

Run these from the repository, and commit whatever they change:

```sh
archdoc link          # resolve any [[...]] links you wrote
archdoc index         # regenerate INDEX.md, which a new document changes
archdoc glossary      # regenerate GLOSSARY.md, if you added or changed a term
archdoc lint          # must exit 0
```

`INDEX.md` and `GLOSSARY.md` are generated. Never edit them by hand. CI checks both are current, and you can run the same checks:

```sh
archdoc index --check
archdoc glossary --check
```

Each prints the difference and exits 2 when the committed file is out of date.

Then rebase onto the configured branch and run the merge rules:

```sh
archdoc lint --merge
```

`--merge` compares the working tree with the merge base of `HEAD` and the configured branch, falling back to `origin/<branch>` where there is no local branch. On a branch that has not been rebased, that is the commit it was cut from, so rebase first to be judged against the branch as it is now. Outside a git repository, or where the branch does not exist, it exits 1 naming the branch.

`archdoc lint` exits 2 when it reports an error and 0 otherwise. Warnings, such as a section left empty in a proposed document, do not change the exit code unless you pass `--strict-warnings`.

## During review

Edits made in review are ordinary commits. Record each one:

- **An RFC** gains an entry in its `Changelog` section for each change, as a dated list item. Continue an entry on lines indented by two spaces.
- **An ADR** has no Changelog. The commit message carries the explanation.

```text
## Changelog

- 2026-10-28: Capped the retry interval at one hour, following review.
```

Lint checks that every Changelog entry is dated, in date order, and within the document's own dates (rule L21). It does not check that an entry exists for each change; that is yours to keep.

## When your number is taken

`archdoc new` takes the next free number at the moment you run it. Someone else branching from the same commit gets the same one, and whichever pull request merges second collides. After you rebase, lint reports it:

```text
rfc/0007-retry-policy-for-scheduled-jobs.md: error: identifier RFC-0007 is also used by rfc/0007-backoff-for-webhooks.md
```

`lint --merge` names the one to renumber, which is always the one arriving:

```text
rfc/0007-retry-policy-for-scheduled-jobs.md:2: error: RFC-0007 is already rfc/0007-backoff-for-webhooks.md's on main; renumber this document instead: archdoc renumber rfc/0007-retry-policy-for-scheduled-jobs.md
```

Your document has not reached the configured branch, so its number is not yet permanent. Renumber it by path:

```sh
archdoc renumber rfc/0007-retry-policy-for-scheduled-jobs.md
archdoc index
archdoc lint
```

`renumber` takes the next free number, rewrites the file name, the `id` and the heading, and follows every reference to the document. Pass the new identifier as a second argument, such as `archdoc renumber rfc/0007-retry-policy-for-scheduled-jobs.md RFC-0009`, to choose the number yourself. Name the document by its path: given `RFC-0007` while two documents carry it, `renumber` lists both and refuses rather than guess. For the same reason, where another document refers to `RFC-0007` by identifier, `renumber` leaves that reference unchanged and lists its line; repoint any that meant your document by hand. `renumber` does not regenerate the index, so run `archdoc index` after it.

`INDEX.md` will usually conflict during the rebase. Resolve it by regenerating it with `archdoc index`, never by merging it by hand.

Never renumber someone else's document to make room for yours.

## The merge rules

`archdoc lint --merge` runs four rules besides the ordinary ones. Each judges what your pull request changes in the record as one step from the configured branch, and each reports an error.

| Rule | Reports | What to do |
| --- | --- | --- |
| M01 | An RFC or ADR arrives as a draft | Run `archdoc propose` on it before it merges. The message names the command with the identifier |
| M02 | A document arrives with an identifier that a document already on the branch carries | Rebase, then `archdoc renumber` the arriving document by path, as above |
| M03 | A change of status the lifecycle does not permit from the status on the branch, judged under that branch's `strict` | Make one step at a time from the branch. With `strict` on, merge the proposal first; the verdict follows separately |
| M04 | An RFC or ADR proposed on the branch has been deleted or renumbered | Restore it, and run `archdoc withdraw` on it if it is no longer wanted |

A document arriving with `backfilled` set may arrive terminal; [Backfilling](backfilling.md) covers those.

## Withdrawing instead of deleting

If you no longer want the document before your pull request merges, close the pull request. Nothing has reached the configured branch, so there is nothing to withdraw.

Once the document is proposed on the configured branch, its number is permanent. If it is no longer wanted, withdraw it:

```sh
archdoc withdraw RFC-0007
```

This sets `status: withdrawn` and `decided` to today. A withdrawn document is terminal and frozen, and keeps its number taken. Never delete or renumber a document that is proposed on the branch: M04 fails the pull request that does. A draft may be deleted.

## What CI runs on your pull request

The workflow `archdoc init` scaffolds runs on every push and pull request. It runs `archdoc lint`, reporting each finding as an annotation on the line that caused it, then `archdoc index --check` and `archdoc glossary --check`. On a pull request into the configured branch, the lint step runs `archdoc lint --merge`. [Checking in CI](ci.md) covers the workflow.

A last step appends `archdoc changes` to the job summary, unless the run was cancelled. It lists each document the pull request adds, changes the status of, edits or removes, compared with the configured branch:

```text
## Changes to the record

| Document | Change |
|---|---|
| RFC-0007: Retry policy for scheduled jobs | Arrives, proposed |
```

You can run `archdoc changes` yourself and paste its output into the pull request's description. It exits 0 whatever it lists; judging the changes is `lint --merge`'s job. `archdoc changes --json` gives the same list as JSON.

## The verdict

Once your pull request merges, your document is proposed on the configured branch and its number is permanent. A maintainer gives the verdict with `archdoc accept` or `archdoc reject`. Do not run either on your own document.

Where `strict` is on, which is the default, the verdict follows once the proposal is on the configured branch, as a commit a maintainer pushes to it or as a pull request of its own, never in the pull request that proposes the document. M03 fails a pull request that brings a document in and accepts or rejects it at once. Where `strict` is off, `draft → accepted` and `draft → rejected` are permitted, and the verdict can come in your pull request.

`archdoc accept` refuses a document with open questions, and both `accept` and `reject` refuse one while lint reports an error about it other than L03, a number another document shares. Reaching a terminal status is a single commit that sets `status` and `decided`, and for a rejection adds the Rejection rationale. The next push to the configured branch freezes the document: lint fails any later change to it, and changing what it says takes a new document that updates or obsoletes it.

## What not to do

- Do not run `archdoc accept` or `archdoc reject` on your own document.
- Do not edit a document that is `accepted`, `rejected` or `withdrawn`. If an existing document is wrong, say so in your own document's Motivation, or open an issue.
- Do not edit `INDEX.md` or `GLOSSARY.md` by hand.
- Do not renumber anyone else's document.
- Do not delete a document that is proposed on the configured branch. Withdraw it.
