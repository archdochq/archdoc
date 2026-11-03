# Process

This repository is the design record and current specification for the project. It is managed with [ArchDoc](https://github.com/archdochq/archdoc). This document describes the document types, their lifecycle, how they relate to one another, and the rules the tooling enforces.

## Document types

There are five types of document, and the glossary's terms, described under Document structure. Each lives in its own directory.

**RFC** (`rfc/`) proposes how something should work. An RFC describes the design of something that will, once built, have a page in the spec. RFCs have a lifecycle and are frozen once they reach a terminal status.

**ADR** (`adr/`) records a decision. An ADR captures a choice that constrains designs but is not itself something the spec would have a page for, together with the alternatives that were considered and rejected. ADRs have the same lifecycle as RFCs and are frozen in the same way.

**Spec** (`spec/`) describes how the project works right now. It is always current. It has no lifecycle and no status; it is edited whenever the behaviour it describes changes. The spec describes and never argues. Argument belongs in the RFCs and ADRs a spec page includes.

**Ref** (`ref/`) holds research, comparisons and background material, or the record of an exchange such as a review round or a meeting. Ref documents are informational, never normative. They carry a verification date rather than a status and may be edited at any time. A record of an exchange also carries the date the exchange took place, and does not go out of date.

**Group** (`group/`) names documents that belong together, such as the documents concerning one application in a repository describing several. A group lists its members and describes what they have in common. It has no status and is never frozen, so a frozen document is grouped by editing the group. Groups are optional; a document may belong to any number of them, or to none.

The test for whether something is an RFC or an ADR: would the spec have a page for it? If yes, it is an RFC. If it is a constraint that applies across the spec rather than a thing the spec describes, it is an ADR. An RFC that contains a significant decision with real rejected alternatives should extract that decision into its own ADR and link to it, so the ADR index remains a complete list of constraints.

## Identity

RFCs, ADRs and refs are numbered. Each type has its own sequence, starting at 1, zero-padded to four digits, and running as far as 9999. The next number is one above the highest that exists, so gaps are permitted and are never filled.

A document that is no longer wanted is withdrawn, not deleted, and a withdrawn draft keeps its number. Deleting the highest-numbered document of a type frees its number, and the next one created takes it, which is the one way a number can be reused.

Identifiers are `RFC-0001`, `ADR-0001` and `REF-0001`. Filenames are the number followed by a slug: `rfc/0001-database.md`. The slug is for humans; the number is the identity. All references between documents use the identifier, never the slug or the filename.

Spec pages are not numbered. They are named by slug: `spec/database.md`. Renaming a spec page is an ordinary edit. Groups are not numbered either, and are named by slug: `group/storage.md`. Both are written by hand.

## Front matter

Every document opens with a YAML front matter block.

RFCs and ADRs share a shape:

```yaml
id: RFC-0004
title: Database component on PostgreSQL
status: draft
created: 2026-09-07
decided:
depends: [RFC-0001, ADR-0002]
updates: [RFC-0001]
obsoletes: []
```

A backfilled RFC or ADR adds one key, which no other document carries:

```yaml
backfilled: 2026-09-11
```

Spec pages:

```yaml
title: Database
includes: [RFC-0001, RFC-0004, ADR-0002, ADR-0003]
```

Refs:

```yaml
id: REF-0001
title: Wings implementations compared
verified: 2026-09-07
```

A record of an exchange adds one key, the date the exchange took place, which is never after `verified`:

```yaml
as_of: 2026-09-04
```

Groups:

```yaml
title: Storage
documents: [RFC-0001, RFC-0004, ADR-0002, database]
```

A member written as an identifier names that RFC, ADR or ref. Any other member names a spec page by its path below `spec/`, without the extension. A group's body is an H1 reading its title, followed by a description of what its members have in common.

The front matter records only forward relationships. Reverse relationships (what updates this, what obsoletes this, which spec pages include this) are derived by the tooling and published in `INDEX.md`. They are never written into the documents themselves, because a frozen document cannot be edited to record what happened to it later.

## Relationships

**`depends`** lists the RFCs and ADRs a document assumes. Read those first. It is a flat list with no typing; it means "this document is written on the assumption that those exist and are accepted".

**`updates`** lists documents this one amends. The older document remains in force except where this one says otherwise. Use this when changing part of an existing design.

**`obsoletes`** lists documents this one replaces entirely. The older document becomes historical and should not be implemented from. Use this when rewriting a design from the ground up.

Both `updates` and `obsoletes` may reference only accepted documents: you can amend or replace only what is in force. A document that was never accepted is superseded by withdrawing it, which is what withdrawal is for.

**`includes`**, on spec pages only, lists the RFCs and ADRs the page currently reflects. A spec page may only include accepted documents. This list is how implementation is recorded: an accepted RFC included by no spec page is not yet implemented; an obsoleted RFC still included by a spec page indicates the spec is stale.

Refs have no relationships of their own. They are cited by identifier from the body of other documents, and may be listed in a group.

**`documents`**, on groups only, lists a group's members, and `INDEX.md` lists each group with its members. The groups a document belongs to are derived from these lists and published in the export.

## Links

Writing `[[RFC-0001]]` or `[[Some Term]]` in a body marks a link to be filled in. `archdoc link` resolves each one to an ordinary Markdown link, pointing at the document with that identifier or at the glossary entry for that term. Lint rejects any that is left unresolved, so a placeholder cannot reach a frozen document.

`archdoc link --suggest` goes the other way, offering to wrap identifiers and glossary terms that appear as plain text. It never rewrites a heading, because a heading's text is how every other rule identifies the section.

## Lifecycle

RFCs and ADRs move through these statuses:

- **`draft`**: being written. May change freely.
- **`proposed`**: open for comment. May change in response to discussion; for an RFC, each change is recorded in its Changelog section.
- **`accepted`**: terminal. The proposal or decision stands.
- **`rejected`**: terminal. Considered on its merits and turned down. The rationale is recorded in a Rejection rationale section.
- **`withdrawn`**: terminal. Pulled by the author before a verdict, because a different approach superseded it or it ceased to be relevant.

The permitted transitions are:

```
draft     → proposed
draft     → withdrawn
proposed  → accepted
proposed  → rejected
proposed  → withdrawn
```

When `strict` is disabled in `archdoc.json`, `draft → accepted` and `draft → rejected` are additionally permitted. By default they are not: a document that was never proposed is a different kind of thing from one that was proposed and accepted the same day, and the record should say which.

A terminal status is final. Once a document reaches `accepted`, `rejected` or `withdrawn` it is never modified again. To change what an accepted document says, write a new document that updates or obsoletes it. To correct a typo in an accepted document, write a new document that updates it, or leave the typo; the record is more valuable than the spelling.

The `decided` field is set to the date the document reached its terminal status and is empty until then.

Implementation is not a status. Whether an accepted RFC has been built is a fact about the spec, recorded through `includes`.

Spec pages and refs have no lifecycle.

## Backfilling

A project that existed before this repository did has a history worth recording. Decisions were taken and designs were built, and the reasoning survives in changelogs, pull requests, issue threads and people's memory. Writing those up afterwards is backfilling.

A backfilled document is created directly in a terminal status. It does not pass through `draft` and `proposed`, because it was never proposed: the decision was taken elsewhere, by some other process or by none, and this repository is recording it rather than making it. Sending it round the lifecycle would fabricate three events that did not happen, which is the same objection that keeps `draft → accepted` out of the default transition graph.

The `backfilled` field carries the date the document was written. `created` and `decided` carry the historical dates: when the work began, and when the decision was taken. A backfilled document therefore states plainly that it was written on one date about a decision taken on another, instead of claiming to have existed all along.

Backfilling has one rule beyond the ordinary ones: **do not invent what was not recorded.** An alternative nobody weighed, or a rationale reconstructed because it sounds convincing, is worse than an admission of ignorance, because a later reader cannot tell it from the real thing. Where the record does not say, write that it does not say. Every backfilled document carries a `Sources` section naming what it was reconstructed from, for the same reason a ref does: the claim needs something it can be checked against.

Numbers are assigned in the order documents are created, not the order the decisions were taken, so a decision from three years ago may carry a higher number than one from last week. When backfilling in bulk at the outset, write the documents in historical order so that the two roughly agree. When backfilling later, do not renumber anything.

A backfilled document is frozen by the same push as any other. Forty of them in one commit freezes forty documents at once, and a mistake in any of them is then permanent. Read them before pushing.

## Document structure

RFCs contain these sections, in this order:

1. **Abstract**: two or three sentences stating what is proposed.
2. **Motivation**: what is wrong or missing today, and why it matters now.
3. **Proposal**: the design.
4. **Alternatives considered**: what else was on the table and why it lost. Significant alternatives that represent a decision in their own right are extracted to an ADR and linked.
5. **Backwards compatibility**: what breaks. May say that nothing does.

Any document may also carry these sections, and the RFC template includes the first and the last:

- **Open questions**: unresolved matters while the document is proposed, each a level three heading with any detail as prose beneath it. Must be absent or empty before the document can be accepted.
- **Resolved questions**: the next section after Open questions. A question answered while proposed moves here with its heading and detail, rather than being deleted, and ends on a blockquote holding its answer, its paragraphs joined by lines holding only `>`. A question handed to a follow-up document is answered with a link to it.
- **Changelog**: one entry for each change made while proposed, as a list item beginning `- YYYY-MM-DD: ` at the start of the line, continued on lines indented by two spaces.

Rejected RFCs and ADRs gain a final **Rejection rationale** section when rejected.

ADRs contain:

1. **Context**: the forces in play.
2. **Decision**: stated as a decision, in one paragraph.
3. **Alternatives**: each with why it lost.
4. **Consequences**: what becomes easier, what becomes harder, what is now constrained.

Spec pages have no fixed structure. They are written in the present tense and describe what the code does. A spec page never argues for the behaviour it describes.

Refs have no fixed structure beyond a closing **Sources** section, so that the `verified` date has something to be checked against.

The glossary is not a spec page. Each term is one file, `term/<slug>.md`, whose front matter carries `title` (the term as written), `formerly` (previous names, oldest first) and `named_by` (the document that introduced the term), and whose body is exactly one paragraph defining it. A term has no status and no lifecycle. A glossary with no entries is valid.

`GLOSSARY.md` is generated from those files, in ascending order of title ignoring case. Every former name keeps an anchor on it, so a link written before a rename still resolves. `spec/glossary.md` is no longer read. `archdoc term` maintains the files, and is easier than writing them by hand.

## Workflow

A document is written on a branch and offered as a pull request into the configured branch. Its author proposes it before asking for review, so it never reaches the configured branch as a draft. Discussion of the document happens on its pull request. Edits made in review to a proposed RFC are ordinary commits, each with an entry in its Changelog. A proposed ADR has no Changelog; its edits are ordinary commits and the commit message carries the explanation.

A document's number becomes permanent when its pull request merges. Two pull requests branched from the same commit can claim the same number, and the second to merge resolves the collision with `archdoc renumber`. A document proposed on the configured branch that is no longer wanted is withdrawn, never deleted or renumbered.

| Transition | Made by |
|---|---|
| `draft → proposed` | The document's author, before asking for review |
| `draft → withdrawn`, `proposed → withdrawn` | The document's author |
| `proposed → accepted`, `proposed → rejected`, and `draft → accepted` and `draft → rejected` where `strict` is off | A maintainer |

ArchDoc does not check who made a change. Who may merge a pull request that accepts or rejects a document is decided by the code host, through branch protection and code owners.

Reaching a terminal status is a single commit that sets `status` and `decided` and, for rejections, adds the Rejection rationale. Where `strict` is on, the verdict follows once the proposal is on the configured branch, as a commit a maintainer pushes to it or a pull request of its own, never in the pull request that proposes the document. The next push is what freezes the document: lint will fail any subsequent change to it.

Spec pages are updated in a commit that references, by URL, the commit or pull request in the code repository that changed the behaviour. The `includes` list is updated in the same commit.

Backfilled documents are written and committed like any other change, except that they arrive already terminal and are frozen by the next push.

## Tooling

ArchDoc provides:

- `archdoc init`: scaffold a new repository in the current directory.
- `archdoc new rfc|adr|ref "Title"`: create the next-numbered document of that type from its template, in draft. `--as-of <date>` creates a ref as a record of an exchange.
- `archdoc propose <id>`, `accept <id>`, `reject <id>`, `withdraw <id>`: move a document through its lifecycle. Each validates the transition, edits the file, and stops. Committing is left to you. `accept` and `reject` refuse a document while lint reports any error about it other than L03, a number another document shares, and `accept` refuses one with open questions, because the next push freezes it and lint would then report a fault nobody is permitted to repair.
- `archdoc lint`: check the repository against the rules in this document. With `--merge`, also judge a pull request, checked out merged into the configured branch, against that branch.
- `archdoc changes`: list what the working tree changes in the record against the configured branch, as Markdown for a pull request's summary, or with `--json`.
- `archdoc link`: resolve `[[...]]` links. With `--suggest`, offer new ones.
- `archdoc renumber <id|path> [new-id]`: change a document's number, rewriting the filename, the `id`, the heading and every reference to it. Takes the next free number when no target is given. Refuses when the document is frozen, or when a frozen document holds a reference it would rewrite.
- `archdoc term add|rename|remove|list|show`: maintain the glossary.
- `archdoc index`: regenerate `INDEX.md`. With `--check`, fail if the committed index is out of date.
- `archdoc glossary`: regenerate `GLOSSARY.md` from `term/`. With `--check`, fail if the committed glossary is out of date.
- `archdoc export`: write the repository as JSON, for anything that renders or indexes it elsewhere. `--schema` prints the schema the output conforms to.
- `archdoc update`: refresh the files ArchDoc generates, including this one, after upgrading it. Shows what would change and asks first.

Lint enforces:

- Every document has valid front matter for its type, with no unknown keys, and no required key or `as_of` left empty.
- Every `id` matches its filename and type; no identifier appears twice.
- Every identifier in `depends`, `updates`, `obsoletes` and `includes` resolves to an existing document of a permitted type.
- Every member of a group resolves to an RFC, ADR, ref or spec page, and no group lists a member twice.
- `includes`, `updates` and `obsoletes` reference only accepted documents. Nothing updates or obsoletes itself, and nothing appears in both lists of one document.
- Documents with a terminal status have `decided` set; others do not, and `decided` is never before `created`.
- A backfilled document is terminal, was written no earlier than the decision it records, and cites its sources.
- A ref's `as_of` is not after its `verified` date.
- The first heading matches the front matter.
- The required sections for the type are present, with their exact titles, in the order listed above, and where the type has a closing section it is the last one.
- No required section is left empty.
- Accepted documents have an empty Open questions section, or none.
- Every question is a level three heading, and every resolved one ends on a blockquote holding its answer.
- Every Changelog entry is a dated list item, in date order and within the document's own dates, written in a form that renders as it is read.
- Any document that is terminal on the base branch is unchanged from the version there. Whether it changed is decided by git rather than by comparing bytes, so a checkout that rewrites line endings does not make every frozen document look edited. Deleting a frozen document is an error; a rename counts as a deletion.
- A spec page is not stale: it includes nothing that has since been obsoleted. Warning.
- Refs whose `verified` date is older than `ref_stale_days` produce a warning, except a record of an exchange, which carries `as_of`.
- Every term's definition is exactly one paragraph.
- No unresolved `[[...]]` link remains anywhere.
- Every relative link resolves to a file that exists, and to a heading or explicit anchor that exists where it carries an anchor.
- Every `depends` entry references a document that is accepted, or one sharing the referencing document's own non-terminal status. Warning.
- No `spec/glossary.md` is left from before terms became files. Warning.

On a pull request into the configured branch, `lint --merge` also enforces, judging each change as one step from that branch:

- No RFC or ADR arrives as a draft.
- A document arriving with an identifier a document on the configured branch already carries is the one to renumber.
- Every change of status is one the lifecycle permits from the status on the configured branch, under that branch's `strict`, or the working tree's where that branch has no `archdoc.json`. A backfilled document may arrive terminal.
- No RFC or ADR proposed on the configured branch is deleted or renumbered. A draft may be.

`INDEX.md` and `GLOSSARY.md` are generated. Do not edit them by hand.

## Configuration

`archdoc.json` holds the only values that vary between projects:

```json
{
  "name": "TheGamePanel",
  "branch": "main",
  "root": ".",
  "strict": true,
  "ref_stale_days": 180
}
```

The process itself is not configurable. A project that needs a different process needs a different tool.
