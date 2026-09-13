---
name: archdoc-classifying
description: Use when deciding whether something belongs in an ArchDoc repository as an RFC, an ADR, a spec page or a ref, and when an RFC should have a decision extracted out of it into its own ADR.
---

# Classifying a document

Four kinds, each in its own directory. Choosing wrongly is the mistake that most shapes a
repository, and it is not something lint can catch.

## The test

PROCESS.md, Document types, gives it directly: **would the spec have a page for it?**

- **Yes**: it is an RFC. An RFC proposes how something should work, and once built, that
  something has a page in the spec.
- **No, it is a constraint that applies across the spec**: it is an ADR. An ADR records a
  choice that constrains designs without being a thing the spec describes.

Two further kinds fall outside that test:

- **Spec page** (`spec/`) describes how the project works *right now*. Always current, no
  lifecycle, no status. Written in the present tense.
- **Ref** (`ref/`) holds research, comparisons and background. Informational, never normative.
  Carries a `verified` date rather than a status.

## Worked distinctions

| Subject | Kind | Why |
| --- | --- | --- |
| How authentication tokens are issued and refreshed | RFC | The spec will have an authentication page |
| That all timestamps are stored in UTC | ADR | A constraint across every design, not a page |
| How the scheduler currently retries failed jobs | Spec page | Describes present behaviour |
| A comparison of three message queues | Ref | Background, informs a decision, is not one |
| That the project targets PostgreSQL rather than MySQL | ADR | Constrains designs, spec pages describe schemas not the choice |
| The design of the new billing pipeline | RFC | Will have a billing page in the spec |

## Extracting an ADR from an RFC

PROCESS.md asks for this explicitly. An RFC that contains a significant decision with real
rejected alternatives should extract that decision into its own ADR and link to it, so the ADR
index remains a complete list of constraints.

The signal is an `Alternatives considered` entry that is doing more work than the rest: a
choice that will constrain future designs beyond this one RFC. Pull it out, create the ADR,
and have the RFC `depends` on it.

If you do not extract it, the constraint is discoverable only by reading an RFC that happens to
mention it, which defeats the purpose of having an ADR index at all.

## When it is genuinely unclear

Prefer an RFC. An RFC that turns out to be a constraint can have an ADR extracted from it
later; an ADR that should have been an RFC leaves the spec with no page to point at, and
nothing signals the gap.

Do not create the document to find out. Ask.
