---
name: archdoc-triage
description: Use when archdoc lint reports findings in an ArchDoc repository, to map each rule to its remedy, decide which document actually needs editing, and recognise the findings that must not be fixed at all.
---

# Reading lint output

```
archdoc lint                    exit 2 if any error
archdoc lint --strict-warnings  exit 2 on a warning too
archdoc lint --json             findings as a JSON array on stdout
```

Findings are printed relative to `root`. Errors fail the build; warnings do not, unless
`--strict-warnings`. The rules are summarised in PROCESS.md, Tooling.

## Before fixing anything: is the document frozen?

**A warning on a frozen document is not a task.** It is a record of something that can no
longer be repaired.

Documents whose status is `accepted`, `rejected` or `withdrawn` are frozen, and lint fails on
any change to one. (PROCESS.md, Lifecycle) A fault inside such a document is reported as a *warning* precisely because
no permitted edit could clear it. Editing the document to silence the warning trades a warning
for an L11 error, which is strictly worse.

This applies to L14, L16 and L17 when the named document is frozen. Leave them. If the content
is genuinely wrong, write a new document that `updates` the frozen one.

## Which document to edit

A finding names the document where the fault was *found*, which is not always where the fix
belongs.

- **L04, L05, L06, L18** are about a relationship. (PROCESS.md, Relationships) The named document references something that
  does not exist, or that is not accepted. The fix is often in the *other* document: accept it,
  or create it. Fixing the named document by deleting the reference is usually wrong.
- **L03** names a duplicate identifier across two documents. Only one of them is wrong; work out
  which and rename that one.
- Everything else is fixed in the document named.

## Rules and remedies

| Rule | What it means | Remedy |
| --- | --- | --- |
| L01 | Front matter is malformed, has an unknown key, or a required key is missing or empty | Fix the front matter |
| L02 | `id` does not match the filename, or the filename is not `<NNNN>-<slug>.md` | Rename the file, or correct `id` |
| L03 | Two documents claim the same identifier | Renumber one with `archdoc new`, then move the content |
| L04 | A referenced identifier does not resolve | Create the document, or correct the reference |
| L05 | `updates`/`obsoletes` targets something not accepted, of the wrong type, or itself | Accept the target, or correct the list |
| L06 | `includes` targets something not accepted | Accept the target, or remove it from the page |
| L07 | `decided` set without a terminal status, or before `created`, or backfill dates disagree | Correct the dates |
| L08 | A required section is missing, misnamed, out of order, or the closing section is not last | Add or reorder. Titles must match exactly |
| L09 | An accepted document still has content under `Open questions` | Move it to a follow-up RFC before accepting |
| L10 | The H1 does not match the front matter | Correct whichever is wrong |
| L11 | A frozen document changed, or was deleted | Revert it. To change what it says, write a new document that updates or obsoletes it |
| L12 | A spec page includes something obsoleted (warning) | Update the page and its `includes` |
| L13 | A ref's `verified` date is older than `ref_stale_days` (warning) | Re-check the content, then update `verified` |
| L14 | A required section is empty | Write it. Only `Open questions` and `Changelog` may be empty |
| L15 | A term's definition under `term/` is not exactly one paragraph | Rewrite the definition as one paragraph |
| L16 | An unresolved `[[...]]` link | Run `archdoc link` |
| L17 | A relative link points at a missing file or heading | Fix the path or the anchor |
| L18 | `depends` names something less settled than the referring document (warning) | Usually accept the dependency first |
| L19 | `spec/glossary.md` is left from before terms became files, and is no longer read (warning) | Move its terms to `term/` with `archdoc term add`. Guide: `spec-pages.md` |

## Two situations that look like faults and are not

**"no commits yet"** on a fresh repository. Nothing can be frozen before there is a branch to
compare against, so L11 is skipped with a warning. Commit, and it goes away.

**`archdoc index --check` or `archdoc glossary --check` failing after any edit.** Both files
are derived. Run `archdoc index` or `archdoc glossary` and commit the result. Never edit
`INDEX.md` or `GLOSSARY.md` by hand.
