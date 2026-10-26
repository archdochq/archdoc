# Backfilling

A project that existed before its specification repository has decisions worth recording. They were taken elsewhere, and the reasoning survives in pull requests, issue threads, changelogs, commit messages and people's memory. Writing those decisions up afterwards is backfilling. This page assumes you know the document types and the lifecycle described in [Concepts](concepts.md).

## What a backfilled document is

A backfilled document is an RFC or ADR created directly in a terminal status: `accepted`, `rejected` or `withdrawn`. It does not pass through `draft` and `proposed`, because it was never proposed in this repository. The decision was taken by some other process, and the repository is recording it rather than making it. Moving it through the lifecycle would record events that did not happen.

Refs, spec pages, terms and groups have no status, so they are never backfilled. Write them as you would at any other time.

## Find the evidence first

Start from what you can find a record of, not from what you remember. For each candidate, decide what the evidence supports:

| What you have | What to write |
|---|---|
| The behaviour, and a record of why it was chosen | A backfilled RFC or ADR, citing that record |
| The behaviour, but no record of why | A spec page, describing what is true now |
| Background reading that informed something | A ref, with a `verified` date |
| A record of an exchange, such as a review round or a meeting | A ref carrying `as_of`, the date it took place |
| Nothing but an inference from the code | Nothing yet: note it as a gap |

A spec page describes present behaviour and needs no historical source. A backfilled ADR claims that a decision was taken and why, and needs a source for that claim. When you know what was built but not why, write the spec page.

Pull request descriptions are usually the richest source, because they tend to state the problem and the alternatives. These commands, run in the code repository, are a place to start:

```sh
git log --merges --format='%h %s'
git log --oneline --no-merges
git log --diff-filter=A --name-only
gh pr list --state merged --limit 50 --json number,title,body,mergedAt
```

Make a list of the documents you intend to write, each with its evidence, before writing any of them.

## Creating a backfilled document

```sh
archdoc new adr "Timestamps are stored in UTC" --backfill --status accepted --created 2024-03-11 --decided 2024-04-02
```

| Flag | Meaning |
|---|---|
| `--backfill` | Create the document already terminal. RFCs and ADRs only. |
| `--status` | `accepted`, `rejected` or `withdrawn`. Defaults to `accepted`. |
| `--created` | The date the work began. No default. |
| `--decided` | The date the decision was taken. Defaults to the `--created` date. |

Dates are written `YYYY-MM-DD`. At a terminal, `archdoc new` prompts for whichever of the status and dates you leave out. Without one, or with `--no-interaction`, a missing `--created` is an error: today is never assumed, because it is never the date of a decision taken before the repository existed.

`archdoc new` refuses:

- a status that is not terminal, such as `--status proposed`;
- `--decided` earlier than `--created`;
- `--created` or `--decided` in the future;
- `--backfill` on a ref, and `--status`, `--created` or `--decided` without `--backfill`.

## What it writes

The front matter carries the historical dates in `created` and `decided`, and today's date in `backfilled`, a key no other document carries:

```yaml
id: ADR-0001
title: Timestamps are stored in UTC
status: accepted
created: 2024-03-11
decided: 2024-04-02
backfilled: 2026-09-13
depends: []
updates: []
obsoletes: []
```

The document states plainly that it was written on one date about a decision taken on another, instead of appearing to have existed all along. `INDEX.md` shows its status as `accepted (backfilled)`.

The body is the usual template for the type, with three differences:

- the alternatives section (Alternatives considered on an RFC, Alternatives on an ADR) holds `Not recorded.`;
- on an RFC, Backwards compatibility also holds `Not recorded.`;
- a `Sources` section is added. On a rejected document, the Rejection rationale section follows it.

Replace `Not recorded.` only where the record does say something. Fill in every other required section from the evidence.

## Which statuses lint accepts

Lint rule L07 holds a backfilled document to its dates and status:

- the status is terminal; `backfilled` on a `draft` or `proposed` document is an error;
- `decided` is set, and is not before `created`;
- `backfilled` is not before `decided`, because a record cannot predate what it records.

Choose the status the record shows. A decision that was taken and stands is `accepted`. One considered and turned down is `rejected`, and needs its Rejection rationale written from the record. Work abandoned before any decision is `withdrawn`.

## Dating from the evidence

`created` is when the work began and `decided` is when the decision was taken. Take both from something you can point at: the date a pull request or issue was opened, the date it was merged or closed, a dated changelog entry, a dated message. Where you can find only one date, give only `--created` and `decided` takes the same value. Do not put today's date, or a guess, in either field.

## Sources

Every backfilled document must carry a `Sources` section naming what it was reconstructed from. Lint requires the section to be present (L08) and not empty (L14). L14 does not check a withdrawn document, so on one of those an empty Sources section goes unreported: fill it in anyway. The section is the document's last H2, except on a rejected document, where the Rejection rationale comes last.

Name specific things a later reader can find: a pull request number or URL, a commit hash, an issue thread, a changelog entry, a dated conversation. "Team knowledge" is not a source.

```markdown
## Sources

- Pull request #412, "Store timestamps in UTC", merged 2024-04-02.
- Commit a3f19c2, the migration converting existing columns.
```

## Do not invent what was not recorded

No lint rule can check this. An alternative nobody weighed, or a rationale reconstructed because it sounds convincing, is worse than an admission that the record is silent, because a later reader cannot tell it from the real thing. Where the record does not say, write that it does not say.

Acceptable:

```markdown
## Alternatives

Not recorded. The pull request discussion mentions a comparison against storing local time with an offset, but does not say what was compared or why UTC was chosen.
```

Not acceptable, however plausible:

```markdown
## Alternatives

Storing local time with an offset was considered but rejected because it complicated sorting.
```

Read each sentence you have written and ask what your source for it is. If you cannot name one, remove the sentence or say that the record does not say.

## Order

A document can only refer to what already exists. `depends`, `updates` and `obsoletes` must name existing documents (L04), and `updates` and `obsoletes` must name accepted documents of the same type (L05). So write a decision before anything that depends on it, amends it or replaces it.

Numbers are assigned in the order documents are created, not the order the decisions were taken. When you backfill in bulk at the outset, write the documents in historical order, within the constraint above, so that numbers and dates roughly agree. When you backfill later, a decision from three years ago may take a higher number than one from last week; leave it, and do not renumber anything to make the order tidy.

## Links

`[[ADR-0001]]` and `[[Some Term]]` placeholders in a backfilled document are resolved by `archdoc link` like any other, as long as the document is not yet on the configured branch. Run it before you commit; lint reports any placeholder left behind (L16).

## Checking and committing

A backfilled document is terminal from the moment it is written, so it freezes as soon as it reaches the configured branch. After that it can never be edited (L11), and a mistake in it is permanent. Before then:

```sh
archdoc link
archdoc index
archdoc lint
```

Lint reports every empty required section in an accepted or rejected backfilled document as an error until it reaches the branch, so `archdoc lint` exits 0 only once each section is written and the Sources are filled in. A withdrawn document's sections are not checked for emptiness, so read those yourself.

Commit in small batches, and read every document before it merges. A batch of forty freezes forty documents at once.

On a pull request, `archdoc lint --merge` accepts a backfilled document arriving already terminal, where it would refuse an ordinary RFC or ADR arriving as accepted with `strict` on. [Contributing by pull request](pull-requests.md) covers the rest of the pull request workflow.
