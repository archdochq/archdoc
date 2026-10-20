---
name: archdoc-contributing
description: Use when submitting a document to an ArchDoc specification repository you do not maintain, by pull request, including what to submit, what to run before pushing, and what to do when someone else takes your number first.
---

# Contributing by pull request

You are offering a document to a repository you do not maintain, as a pull request. You write
it, propose it before asking for review, and withdraw it if you no longer want it. Accepting and
rejecting it are the maintainers' to do. (PROCESS.md, Workflow)

## Before you write anything

Decide what kind of document this is. An RFC proposes how something should work and will have
a spec page once built; an ADR records a constraint that applies across the spec. Getting this
wrong wastes the review. See `classifying.md`.

You need `archdoc` on your PATH. Either download the binary for your platform from the
[releases page](https://github.com/archdochq/archdoc/releases), or, with Go installed, run
`go install archdoc.dev/cmd/archdoc@latest`.

## Writing it

Fork, clone, and branch. Then:

```
archdoc new rfc "Retry policy for scheduled jobs"
```

Never create the file by hand. The filename encodes the identity and lint rejects one that does
not match. (PROCESS.md, Identity)

Fill every required section. `authoring.md` lists them per type and describes the standard they
have to meet.

When it is ready for comment, propose it:

```
archdoc propose RFC-0007
```

A pull request is where a document is offered for comment, so a draft cannot merge: the
repository's CI runs `archdoc lint --merge`, which fails on a document arriving as a draft.

## Before you push

```
archdoc link        resolve any [[...]] you wrote
archdoc index       regenerate INDEX.md, which your new document changes
archdoc lint        must exit 0
archdoc lint --merge   once rebased onto the target branch, also exits 0
```

`archdoc index` is not optional. Adding a document changes the generated index, and the
repository's CI runs `archdoc index --check` on your pull request, which fails if you did not
commit the regenerated file. Do not edit `INDEX.md` by hand.

## Your number may not survive review

`archdoc new` takes the next number free **at the moment you run it**. Someone else branching
from the same commit gets the same one, and whichever pull request merges second collides.

Lint reports it clearly:

```
rfc/0002-alice.md: error: identifier RFC-0002 is also used by rfc/0002-bob.md
```

On the pull request, `lint --merge` names which of the two to renumber: the one arriving, since
the other already holds the number on the branch. This is expected and cheap to fix, because
your document has not reached the branch, so its number is not yet permanent and nothing about
it is frozen. Rebase on the updated configured branch, then:

```
archdoc renumber rfc/0002-yours.md
archdoc index
archdoc lint
```

**Name your document by path, not by identifier.** During a collision two documents carry
`RFC-0002`, so `archdoc renumber RFC-0002` cannot tell which one you mean. It refuses and lists
both rather than guessing, because guessing would renumber the other contributor's work.

`renumber` takes the next free number, rewrites the filename, the `id` and the heading, and
follows every reference to the old identifier. Pass a second argument to choose the number
yourself.

Expect `INDEX.md` to conflict on the rebase. Regenerating it is the resolution, never a hand
merge.

## What not to do

- **Do not run `archdoc accept` or `reject`.** Those record a verdict, and the verdict is not
  yours. (PROCESS.md, Workflow)
- **Do not edit anyone else's document.** Anything already `accepted`, `rejected` or
  `withdrawn` is frozen and changing it fails lint. If an existing document is wrong, say so in
  your own document's Motivation, or open an issue.
- **Do not edit `INDEX.md` by hand.** Regenerate it.
- **Do not renumber anyone else's document** to make room for yours.

## During review

Discussion of your document happens on its pull request. Record each change you make in review:
an RFC gains an entry in its `Changelog` for each one, and an ADR, which has no Changelog,
carries the explanation in the commit message. (PROCESS.md, Workflow)

If you no longer want the document before it merges, close the pull request. Once it is on the
branch, it is withdrawn rather than deleted, which keeps its number taken:

```
archdoc withdraw RFC-0007
```

## What happens next

Your pull request runs `archdoc lint --merge`, `archdoc index --check` and `archdoc changes`,
which summarises what it does to the record. The checks must pass. Review is about the content;
the tooling only checks the shape.

Once it merges, your document is proposed on the branch and its number is permanent. A
maintainer gives the verdict. Where `strict` is on, the default, that follows the merge, as a
commit or a pull request of its own; where it is off, it can come in your pull request. Once the
document reaches a terminal status on the branch it is frozen permanently. (PROCESS.md, Workflow)
