# Concepts

ArchDoc manages a specification repository: a directory of Markdown documents, each opening with a YAML front matter block, held in git. This page describes what the repository holds and the rules the tool applies to it. Installing the tool is covered in [Installing ArchDoc](installing.md), and a first repository in [Five minutes with ArchDoc](five-minutes.md).

## The repository

A specification repository is the directory holding `archdoc.json`. Every command that works on a repository, which is every command but `archdoc init` and `archdoc reference`, finds that file by walking up from the current directory, so you can run commands from anywhere inside the repository. `-C <dir>` runs a command as if it had been started in another directory.

The documents live in one directory per type, under the directory `archdoc.json` names as `root`: `rfc/`, `adr/`, `spec/`, `ref/`, `term/` and `group/`. Every `*.md` file in one of those directories is a document of that type, and files anywhere else are not documents. `spec/` may have subdirectories, each page addressed by its path below `spec/`; subdirectories of the other directories are ignored. `archdoc init` creates `rfc/`, `adr/`, `ref/` and `term/`; you create `spec/` and `group/` when you need them.

The same directory holds `README.md`, `PROCESS.md` (the rules this page summarises, written by `archdoc init` and refreshed by `archdoc update`), and the two generated files, `INDEX.md` and `GLOSSARY.md`.

## Document types

There are six types.

| Type | Directory | Named by | Status | Frozen |
|---|---|---|---|---|
| RFC | `rfc/` | number, `RFC-0001` | yes | once terminal on the configured branch |
| ADR | `adr/` | number, `ADR-0001` | yes | once terminal on the configured branch |
| Spec page | `spec/` | path, `database` or `http/routing` | no | never |
| Ref | `ref/` | number, `REF-0001` | no | never |
| Term | `term/` | file name, `tenant` | no | never |
| Group | `group/` | file name, `storage` | no | never |

### RFC

An RFC proposes how something should work. It describes the design of something that will, once built, have a page in the spec. Its required sections, in order, are Abstract, Motivation, Proposal, Alternatives considered and Backwards compatibility.

### ADR

An ADR records one decision: a choice that constrains designs but is not itself something the spec would have a page for, together with the alternatives that were rejected. Its required sections, in order, are Context, Decision, Alternatives and Consequences.

To choose between the two, ask whether the spec would have a page for it. If it would, write an RFC. If it is a constraint that applies across the spec, write an ADR. An RFC that contains a significant decision with real rejected alternatives should extract that decision into its own ADR and link to it.

### Spec page

A spec page describes how the project works now, in the present tense. It has no status, no fixed structure, and is edited whenever the behaviour it describes changes. It never argues for that behaviour: the argument belongs in the RFCs and ADRs it includes. Spec pages are written by hand, not with `archdoc new`.

### Ref

A ref holds research, comparisons and background, or the record of an exchange such as a review round or a meeting. It is informational, never normative. It carries a `verified` date instead of a status, may be edited at any time, and ends with a `Sources` section. A ref that records an exchange also carries `as_of`, the date the exchange took place (`archdoc new ref "Title" --as-of 2026-09-04`), which may not be after `verified`.

### Term

A term is one glossary entry: a file `term/<slug>.md` whose front matter holds `title` (the term as written), `formerly` (previous names, oldest first) and `named_by` (the document that introduced it), and whose body is an H1 holding the term followed by exactly one paragraph defining it. `archdoc term add`, `rename`, `remove`, `list` and `show` maintain these files.

### Group

A group names documents that belong together, such as the documents concerning one application in a repository describing several. Its `documents` list holds identifiers of RFCs, ADRs and refs, and spec page paths below `spec/` without the extension. Its body is an H1 reading its title and a description of what the members share. A group has no status and is never frozen, so a frozen document can still be added to a group by editing the group. Groups are optional, and are written by hand.

## Numbering

RFCs, ADRs and refs are numbered, each type in its own sequence starting at 1 and running to 9999. `archdoc new rfc|adr|ref "Title"` creates the next one: one above the highest number in that directory, so gaps are permitted and never filled. The filename is the number and a slug of the title, `rfc/0004-database-component.md`, and the `id` in the front matter must match it. The number is the identity: references between documents use the identifier, never the filename.

A document that is no longer wanted is withdrawn, not deleted. `archdoc renumber <id|path> [new-id]` changes the number of a document that is not frozen, rewriting the file name, the `id`, the heading and every reference to it; [Contributing by pull request](pull-requests.md) covers when you need it.

## The lifecycle

RFCs and ADRs have a `status`. No other type does.

| Status | Meaning |
|---|---|
| `draft` | Being written. May change freely. |
| `proposed` | Open for comment. May change in response to review; for an RFC, each change gets an entry in its Changelog section. |
| `accepted` | Terminal. The proposal or decision stands. |
| `rejected` | Terminal. Considered and turned down. The document gains a final Rejection rationale section. |
| `withdrawn` | Terminal. Pulled by its author before a verdict. |

There are no other statuses. Whether a document is obsolete, or a spec page stale, is derived from relationships (see [Derived facts](#derived-facts)), not written as a status.

The permitted transitions are:

```text
draft     -> proposed
draft     -> withdrawn
proposed  -> accepted
proposed  -> rejected
proposed  -> withdrawn
```

`archdoc propose <id>`, `accept <id>`, `reject <id>` and `withdraw <id>` make these moves. Each checks the transition, rewrites `status`, sets `decided` to today on a terminal move, and stops; committing is left to you. `reject` appends a Rejection rationale section for you to fill in. `accept` refuses a document with anything left under Open questions, and `accept` and `reject` refuse a document that lint reports an error about (other than L03, a shared number).

### Strict

`strict` in `archdoc.json` is `true` by default. With it off, two more transitions are permitted: `draft -> accepted` and `draft -> rejected`. Nothing else changes. With it on, a document must be proposed before it can be decided, so the record shows whether it was ever open for comment.

On a pull request, the merge rules judge each change of status under the `strict` of the configured branch's `archdoc.json`. Where it is on, a pull request may not bring in an RFC or ADR already accepted or rejected, unless it is backfilled: propose it in one pull request, and give the verdict once that has merged.

## Freezing

A document with a terminal status is frozen once it is on the configured branch, the `branch` in `archdoc.json`. From then on lint reports any change to it (rule L11), and deleting or renaming it is reported too. Whether it changed is decided by git, against the merge base of `HEAD` and the local branch, or `origin/<branch>` where there is no local one, so a branch cut before a document was accepted is not reported for holding the earlier version. A document that is terminal in your working tree but not yet on the branch is not frozen, so you can still correct it before it merges.

To change what a frozen document says, write a new document that updates or obsoletes it. To correct a typo in one, write a new document that updates it, or leave the typo.

Spec pages, refs, terms and groups are never frozen.

## Relationships

Front matter records only forward relationships.

- `depends`, on an RFC or ADR: the RFCs and ADRs this document assumes exist and are accepted. Read those first.
- `updates`, on an RFC or ADR: documents of the same type this one amends. The older document stays in force except where this one says otherwise.
- `obsoletes`, on an RFC or ADR: documents of the same type this one replaces entirely. The older document becomes historical.
- `includes`, on a spec page: the RFCs and ADRs the page currently reflects. This is how implementation is recorded.
- `documents`, on a group: its members.

`updates`, `obsoletes` and `includes` may only name accepted documents: you can amend, replace or implement only what is in force. A document that was never accepted is set aside by withdrawing it.

Reverse relationships (what updates a document, what obsoletes it, which spec pages include it) are never written into documents, because a frozen document could not be edited to record them. ArchDoc derives them by reading the whole repository.

### Derived facts

- A document is **effectively obsolete** when an accepted document lists it in `obsoletes`. Obsolescence claimed by a document in any other status has no effect.
- An RFC or ADR is **implemented** when it is accepted and some spec page includes it.
- A spec page is **stale** when it includes a document that is effectively obsolete.

## Links and the glossary

Writing `[[RFC-0001]]` or `[[Tenant]]` in a body marks a link to be filled in. `archdoc link` replaces each one with an ordinary Markdown link, to the document with that identifier or to the term's entry in `GLOSSARY.md`; where the text matches both, the term wins. Lint rejects a `[[...]]` left unresolved. `archdoc link --suggest` offers to link identifiers and terms that appear as plain text, and never rewrites a heading.

## Generated files

`INDEX.md` and `GLOSSARY.md` are generated. Do not edit them by hand.

`archdoc index` writes `INDEX.md`: a table each for RFCs, ADRs, spec pages and refs, and one for groups where any exist. The RFC and ADR tables show each document's status, decided date, what it updates and obsoletes, what updates and obsoletes it, and the spec pages it is implemented in. A status carries qualifiers in parentheses, such as `accepted (backfilled)` or `accepted (obsolete)`. The spec table marks stale pages.

`archdoc glossary` writes `GLOSSARY.md` from the files under `term/`, sorted by title ignoring case. Every former name of a term keeps an anchor on the page, so a link written before a rename still resolves.

Neither file is regenerated by other commands: `archdoc new` and `archdoc term add` leave them as they were. Run the generator after a change, and commit the result. `archdoc index --check` and `archdoc glossary --check` print the difference and exit 2 where the committed file is out of date, which is what the scaffolded CI workflow runs; see [Checking in CI](ci.md).

## Backfilled documents

A backfilled RFC or ADR records a decision taken before the repository existed. It is created directly in a terminal status with `archdoc new --backfill`, carries a `backfilled` date beside its historical `created` and `decided` dates, and must have a `Sources` section. [Backfilling](backfilling.md) covers writing one.

## Lint

`archdoc lint` checks every document and prints one finding per line:

```text
adr/0001-timestamps-are-stored-in-utc.md:19: error: section "Decision" is empty
```

It exits 2 if any finding is an error, 1 when it cannot run at all, as with no `archdoc.json` or a flag it does not know, and 0 otherwise. Warnings do not change the exit code unless you pass `--strict-warnings`. `--json` prints the findings as a JSON array.

The L rules, L01 to L21, check a document on its own and against the rest of the repository: front matter and identifiers (L01 to L03), relationships (L04 to L06, L18), dates (L07), required sections and headings (L08 to L10, L14), freezing (L11), staleness of spec pages and refs (L12, L13), terms (L15, L19), links (L16, L17), and the shape of question and Changelog sections (L20, L21). A fault in a frozen document that no permitted edit could clear is reported as a warning rather than an error.

The M rules, M01 to M04, run only with `archdoc lint --merge`, on a pull request checked out merged into the configured branch. They judge each change against that branch: no RFC or ADR arrives as a draft (M01), an arriving document whose number is already taken is the one renumbered (M02), every change of status is a permitted transition (M03), and a document proposed on the branch is withdrawn rather than deleted or renumbered (M04).

`archdoc changes` lists what the working tree changes in the record against the configured branch, as a Markdown table or, with `--json`, as JSON.

## archdoc.json

`archdoc.json` holds the only settings that vary between projects. The process itself is not configurable.

```json
{
  "name": "Example",
  "branch": "main",
  "root": ".",
  "strict": true,
  "ref_stale_days": 180
}
```

| Key | Default | Meaning |
|---|---|---|
| `name` | none, required | The project name, used in the headings of `README.md` and `INDEX.md`. |
| `branch` | `main` | The branch frozen documents are compared against, and that pull requests are judged against. |
| `root` | `.` | The directory, relative to `archdoc.json`, holding the document directories and generated files. It must not be absolute or lead outside the directory holding `archdoc.json`. |
| `strict` | `true` | Whether `draft -> accepted` and `draft -> rejected` are refused. |
| `ref_stale_days` | `180` | How old a ref's `verified` date may be before lint warns (L13). A ref carrying `as_of` is not reported. `0` turns the check off. |

An unknown key is an error, and a missing key takes its default. `archdoc init` writes the file, prompting for each value at a terminal or taking them from `--name`, `--branch`, `--root`, `--strict` or `--no-strict`, and `--ref-stale-days`. A `root` other than `.` is how a specification lives inside the code repository it documents; see [A spec repository beside the code](beside-the-code.md).
