---
name: archdoc-contributing
description: Use when submitting a document to an ArchDoc specification repository you do not maintain, by pull request, including what to submit, what to run before pushing, and what to do when someone else takes your number first.
---

# Contributing by pull request

You are adding a **draft** to a repository you do not maintain. That is the whole of what a
contribution is. Proposing it, accepting it and rejecting it are the maintainers' to do.
(PROCESS.md, Workflow)

## Before you write anything

Decide what kind of document this is. An RFC proposes how something should work and will have
a spec page once built; an ADR records a constraint that applies across the spec. Getting this
wrong wastes the review. See `classifying.md`.

You need `archdoc` on your PATH. Either download the binary for your platform from the
[releases page](https://github.com/archdochq/archdoc/releases), or, with Go installed, run
`go install github.com/archdochq/archdoc/cmd/archdoc@latest`.

## Writing it

Fork, clone, and branch. Then:

```
archdoc new rfc "Retry policy for scheduled jobs"
```

Never create the file by hand. The filename encodes the identity and lint rejects one that does
not match. (PROCESS.md, Identity)

Fill every required section. `authoring.md` lists them per type and describes the standard they
have to meet. Leave `Changelog` empty: entries are added while the document is proposed, which
has not happened yet.

## Before you push

```
archdoc link        resolve any [[...]] you wrote
archdoc index       regenerate INDEX.md, which your new document changes
archdoc lint        must exit 0
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

This is expected and cheap to fix, because your document is still a draft and nothing about a
draft is frozen. Rebase on the updated main branch, then:

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

- **Do not run `archdoc propose`, `accept` or `reject`.** Those record a verdict, and the
  verdict is not yours. Submit the document as a draft and let the maintainers move it.
  (PROCESS.md, Lifecycle)
- **Do not edit anyone else's document.** Anything already `accepted`, `rejected` or
  `withdrawn` is frozen and changing it fails lint. If an existing document is wrong, say so in
  your own document's Motivation, or open an issue.
- **Do not edit `INDEX.md` by hand.** Regenerate it.
- **Do not renumber anyone else's document** to make room for yours.

## What happens next

Your pull request runs `archdoc lint` and `archdoc index --check`. Both must pass. Review is
about the content; the tooling only checks the shape.

If it is merged, a maintainer moves it to `proposed` when it is ready for comment. From that
point every change to it is recorded in its `Changelog`, and when it reaches a terminal status
it is frozen permanently. (PROCESS.md, Lifecycle)
