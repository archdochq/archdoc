---
title: Lint
includes: [ADR-0008, ADR-0009, ADR-0010, RFC-0008, RFC-0009, RFC-0011, RFC-0013, RFC-0014, RFC-0015]
---

# Lint

`archdoc lint` runs every L rule over every document and prints findings. With `--merge` it also runs the merge rules. Exit code 2 if any error, 0 otherwise. Warnings do not affect the exit code unless `--strict-warnings` is given, in which case a warning alone exits 2.

## Severity

Every rule reports an error unless stated. A finding that no permitted edit to the named document could clear is a warning. This applies to L14, L16, L17, L20 and L21, whose subject is the document's own body, when the document is frozen. It does not apply to a fault in a relationship between documents, which is cleared by editing the other document, nor to L11.

## Rules

- **L01** Front matter parses and matches the schema for the type: no unknown keys, no missing required keys, no wrong types, no required key left empty, and no `as_of` left empty.
- **L02** `id` equals the identifier derived from type and filename. The filename matches `<NNNN>-<slug>.md`.
- **L03** No identifier appears on more than one document.
- **L04** Every identifier in `depends`, `updates`, `obsoletes` and `includes` resolves to an existing document of a permitted type. Every member of a group resolves, an identifier to an RFC, ADR or ref and any other member to a spec page, and no group lists a member twice. Each finding is reported at the line of the list. A document in no group is not reported.
- **L05** `updates` and `obsoletes` reference only accepted documents of the same type. No document updates or obsoletes itself. No document appears in both lists of the same document.
- **L06** `includes` references only accepted documents.
- **L07** `decided` is set if and only if the status is terminal. `decided` is not before `created`. If `backfilled` is set, the status is terminal and `backfilled` is not before `decided`. On a ref, `as_of` is not after `verified`.
- **L08** Required sections are present, with exact titles, in the listed relative order. Other H2 sections are permitted between or after them. `Rejection rationale` is present if and only if the document is rejected. Where the type has a closing section, it is the document's last H2.
- **L09** An accepted document has no non-empty `Open questions` section.
- **L10** The H1 matches the front matter.
- **L11** Frozen documents are unchanged. For every document whose status on `branch` is terminal, the working tree file is unchanged from the file at `branch`, as git judges it. A file absent from `branch` is new and the rule does not apply. A document terminal on `branch` and absent from the working tree is an error; a rename is a deletion and a new file. With no commits, the rule is skipped with a warning. When `branch` does not exist in a repository that has commits, the rule reports an error naming the branch. Outside a git repository, the rule is skipped with a warning.
- **L12** A spec page includes no document that is effectively obsolete. Warning.
- **L13** A ref's `verified` date is not older than `ref_stale_days`. A ref carrying `as_of` is not reported. Warning.
- **L14** No required section is empty. Warning for `proposed`; error for a document that is terminal in the working tree but not yet on `branch`; warning once frozen. Not applied to `withdrawn`.
- **L15** Term structure: exactly one paragraph of definition. No terms is valid.
- **L19** A `spec/glossary.md` left behind by a repository scaffolded before terms became files. A warning: the page is valid Markdown and nothing else is wrong with the repository.
- **L16** No unresolved `[[...]]` wiki link anywhere. Error in an editable document; warning in a frozen one.
- **L17** Every relative Markdown link resolves to an existing file within the repository, and where it has an anchor, to an existing heading. Error in an editable document; warning in a frozen one.
- **L18** Every `depends` entry references a document that is accepted, or that shares the referencing document's non-terminal status. Warning.
- **L20** The question sections have the shape [Markdown](markdown.md) gives them. Reported at the first line before a section's first question that is not blank or a comment; at a question whose heading produces no anchor, or the same anchor as another heading, and of two questions at the later; at a heading beneath a question; at a blockquote line holding a heading, a comment, or a fence's opening line, the lines within a quoted fence being code; at an open question's final blockquote; at an empty `Resolved questions` section's heading, or, in a document that has an `Open questions` section, one that does not follow it; at a resolved question with no heading beneath it that does not end with a blockquote; at an answer with another blockquote before it, or an empty one; and at each `Resolved questions` heading after the first, whose questions and blockquotes are checked as the first section's are and whose section is otherwise not checked. Not applied to the `Open questions` section of an accepted document, which L09 checks, or to a withdrawn document. Error in an editable document; warning in a frozen one.
- **L21** Every `Changelog` section has the shape [Markdown](markdown.md) gives it. Reported at the first line belonging to no entry after the section's heading and after each entry; at a heading opening or ending the section that is indented by two or three spaces, or is a heading only once a comment on its line is masked; at an entry with no date, one beginning with a date shape not followed by `:` and a space or the end of the line, one whose date is not a real date, and one with a date and no text; at an entry dated earlier than the dated entry before it, before a valid `created`, or after `decided`, the last two unchecked where `decided` is before `created`; at each `Changelog` heading after the first; and at a level two heading differing from `Changelog` only in case, spacing or punctuation. Not applied to a withdrawn document. Error in an editable document; warning in a frozen one.

## Merge rules

`archdoc lint --merge` also runs M01 to M04, which compare the working tree with the base: the merge base of HEAD and the configured branch, the branch resolved as L11 resolves it, with the same fallback to `origin/<branch>`. In a checkout of a pull request merged into the branch, that is the branch as the merge was made; on a branch not rebased since it was cut, it is the commit it was cut from. Every document on the base, and the base's `archdoc.json`, are read in one batch from its file listing. Outside a git repository, where the branch does not exist, or where HEAD shares no history with it, `--merge` exits 1 naming the branch and reports nothing. Without `--merge` no merge rule runs, including in the transition commands. The merge rules report alongside the L rules, with the same output, `--json`, `--strict-warnings` and exit codes, and every one reports an error.

A document in the working tree was on the base when the base holds a document at its path, or, for a numbered document, one carrying its identifier at a path absent from the working tree, which is a rename. Any other document arrives.

- **M01** An RFC or ADR arrives as a draft. Reported at its `status` line, naming `archdoc propose` with its identifier where it has one.
- **M02** A numbered document arrives carrying an identifier that a document on the base, still in the working tree, carries. Reported at the arriving document's `id` line, naming the other document's path and `archdoc renumber` with the arriving document's path.
- **M03** An RFC or ADR's status differs from its status on the base by a transition [Lifecycle](lifecycle.md) does not permit, under the `strict` of the base's `archdoc.json`, or of the working tree's where the base holds none. An arriving document is judged as moving from `draft`, except an arriving draft, which M01 reports, and one carrying `backfilled`, which L07 requires to be terminal. A document terminal on the base, and a status on either side that is not one of the five, are not judged. Reported at the `status` line; where the base's `strict` is on and the move is from `draft`, the message says to merge the proposal first.
- **M04** An RFC or ADR proposed on the base whose identifier no document in the working tree carries: deleted, or renumbered. Reported at its path on the base, naming `archdoc withdraw`. A draft may be deleted, and a terminal document's deletion is L11's.

## Rule entries

Each rule in the registry carries, besides its code and check:

| Field | Content |
|---|---|
| `Title` | A few words naming what the rule checks |
| `Summary` | What a finding means, as inline Markdown |
| `Remedy` | How a finding is cleared, as inline Markdown |
| `Since` | The tag of the first release carrying the rule: `v0.1.0-alpha.1` for L01 to L18, `v0.5.0` for L19, `v0.6.0` for L20, L21 and M01 to M04 |
| `Changes` | Each later release that changed what the rule reports, with a one-line account, in release order. Changes made before the registry recorded them are not listed |

A test refuses an entry without a title, summary, remedy or `since`. The table of rules in the triage guide is the registry's summaries and remedies, one row per rule in code order; a test compares the two, and `go test ./internal/template -update-triage` rewrites the table. `archdoc reference` publishes every entry.

## Output

One finding per line: `<path>:<line>: <severity>: <message>` where a line is known, `<path>: <severity>: <message>` otherwise. `<path>` is relative to `root`. Findings are sorted by path, then line, then rule. With `--json`, a bare JSON array of objects with `path`, `line`, `severity`, `message` and `rule`; `line` is omitted where unknown.
