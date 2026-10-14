---
name: archdoc-authoring
description: Use when writing a new RFC or ADR in an ArchDoc repository and moving it through draft, proposed and a terminal status, including which sections are required and what must be true before it freezes.
---

# Authoring a document

## Create it with the tool

```
archdoc new rfc "Retry policy for scheduled jobs"
```

Never write the file yourself. The number is the identity, the tool assigns it as one above the
highest that exists, and a filename that does not match `<NNNN>-<slug>.md` fails lint.
(PROCESS.md, Identity)

The title reaches the H1 and the front matter, and lint checks the two agree. Changing the
title later means editing both; the slug in the filename is left alone and that is expected.

## Required sections

Exact H2 titles, in this order. (PROCESS.md, Document structure)

**RFC**

1. `Abstract` two or three sentences stating what is proposed
2. `Motivation` what is wrong or missing today, and why it matters now
3. `Proposal` the design
4. `Alternatives considered` what else was on the table and why it lost
5. `Backwards compatibility` what breaks, or that nothing does

`Open questions` and `Changelog` are optional, and the RFC template includes both.

**ADR**

1. `Context` the forces in play
2. `Decision` stated as a decision, in one paragraph
3. `Alternatives` each with why it lost
4. `Consequences` what becomes easier, what becomes harder, what is now constrained

Extra H2 sections are allowed between or after the required ones. Subsections are yours to
choose.

## Filling them

Every required section must be non-empty by the time the document is accepted.

A section containing only the template's HTML comment counts as empty. Replace the comment;
do not write around it.

`Backwards compatibility` saying "Nothing breaks. This is new behaviour with no existing
callers." is a complete answer. An empty section is not.

## Questions

Each open question is a level three heading under `Open questions`, with any detail as prose
beneath it and no heading beneath it. `Open questions` must be empty, or gone, before the
document is accepted.

A question answered while proposed is moved, not deleted. Its heading and detail go to
`Resolved questions`, the next level two heading after `Open questions`, and its answer is
added after them as a blockquote. A blank line ends a blockquote, so join the answer's
paragraphs with a line holding only `>`:

```markdown
## Resolved questions

### Should `index.json` carry the questions?

Raised in review.

> Yes, without the bodies.
>
> Each document's own file carries them.
```

A question handed to a follow-up document is answered with a link to it. An open question
never ends with a blockquote, and nothing in a blockquote there is a heading, a fence or a
comment. (PROCESS.md, Document structure)

## The Changelog

Each change made while the document is proposed gets an entry: a list item at the start of the
line, dated, with any further lines indented by two spaces.

```markdown
## Changelog

- 2026-09-02: Moved the question on `index.json` to Resolved
  questions, with its answer.
```

Nothing else goes in the section but blank lines and comments. Dates are in order, and fall
between the document's `created` and `decided`.

## Links between documents

Write `[[RFC-0001]]` or `[[Some Term]]` where a link belongs, then run `archdoc link` to
resolve them into ordinary Markdown links. Lint rejects any left unresolved.
(PROCESS.md, Links)

`archdoc link --suggest` goes the other way and offers to wrap identifiers and glossary terms
already sitting in your prose. It never rewrites a heading.

Relationships go in the front matter, not the prose: `depends` for what must be read first,
`updates` for what this amends, `obsoletes` for what it replaces. `updates` and `obsoletes`
may only reference accepted documents. (PROCESS.md, Relationships)

## Moving it through the lifecycle

```
archdoc propose <id>     draft -> proposed
archdoc accept <id>      proposed -> accepted
archdoc reject <id>      proposed -> rejected, then write the Rejection rationale
archdoc withdraw <id>    pulled before a verdict
```

By default `draft` cannot go straight to `accepted`. That is deliberate: a document that was
never proposed is a different kind of thing from one proposed and accepted the same day.
(PROCESS.md, Lifecycle)

## The point of no return

**The push after a terminal transition freezes the document permanently.** Before it, every
fault is repairable. After it, none are, and lint will report them forever.

`accept` and `reject` refuse a document with an empty required section, an unresolved
`[[...]]` link, or a question or Changelog entry of a shape lint reports, and `accept` refuses
one with open questions, which catches the common cases. They cannot catch a section that is present,
non-empty and wrong.

So before the commit that sets a terminal status, read the document. Check the H1 matches the
title, every section says something, and the links point where you meant. That is the last
moment it can be changed.
