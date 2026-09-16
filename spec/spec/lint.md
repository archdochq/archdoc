---
title: Lint
includes: [ADR-0008, ADR-0009, ADR-0010]
---

# Lint

`archdoc lint` runs every rule over every document and prints findings. Exit code 2 if any error, 0 otherwise. Warnings do not affect the exit code unless `--strict-warnings` is given, in which case a warning alone exits 2.

## Severity

Every rule reports an error unless stated. A finding that no permitted edit to the named document could clear is a warning. This applies to L14, L16 and L17, whose subject is the document's own body, when the document is frozen. It does not apply to a fault in a relationship between documents, which is cleared by editing the other document, nor to L11.

## Rules

- **L01** Front matter parses and matches the schema for the type: no unknown keys, no missing required keys, no wrong types, no required key left empty.
- **L02** `id` equals the identifier derived from type and filename. The filename matches `<NNNN>-<slug>.md`.
- **L03** No identifier appears on more than one document.
- **L04** Every identifier in `depends`, `updates`, `obsoletes` and `includes` resolves to an existing document of a permitted type.
- **L05** `updates` and `obsoletes` reference only accepted documents of the same type. No document updates or obsoletes itself. No document appears in both lists of the same document.
- **L06** `includes` references only accepted documents.
- **L07** `decided` is set if and only if the status is terminal. `decided` is not before `created`. If `backfilled` is set, the status is terminal and `backfilled` is not before `decided`.
- **L08** Required sections are present, with exact titles, in the listed relative order. Other H2 sections are permitted between or after them. `Rejection rationale` is present if and only if the document is rejected. Where the type has a closing section, it is the document's last H2.
- **L09** An accepted document has no non-empty `Open questions` section.
- **L10** The H1 matches the front matter.
- **L11** Frozen documents are unchanged. For every document whose status on `branch` is terminal, the working tree file is unchanged from the file at `branch`, as git judges it. A file absent from `branch` is new and the rule does not apply. A document terminal on `branch` and absent from the working tree is an error; a rename is a deletion and a new file. With no commits, the rule is skipped with a warning. When `branch` does not exist in a repository that has commits, the rule reports an error naming the branch. Outside a git repository, the rule is skipped with a warning.
- **L12** A spec page includes no document that is effectively obsolete. Warning.
- **L13** A ref's `verified` date is not older than `ref_stale_days`. Warning.
- **L14** No required section is empty, except `Open questions` and `Changelog`. Warning for `proposed`; error for a document that is terminal in the working tree but not yet on `branch`; warning once frozen. Not applied to `withdrawn`.
- **L15** Glossary structure: after the preamble, H2 entries only, unique, in ascending case-insensitive alphabetical order, one paragraph each. No entries is valid.
- **L16** No unresolved `[[...]]` wiki link anywhere. Error in an editable document; warning in a frozen one.
- **L17** Every relative Markdown link resolves to an existing file within the repository, and where it has an anchor, to an existing heading. Error in an editable document; warning in a frozen one.
- **L18** Every `depends` entry references a document that is accepted, or that shares the referencing document's non-terminal status. Warning.

## Output

One finding per line: `<path>:<line>: <severity>: <message>` where a line is known, `<path>: <severity>: <message>` otherwise. `<path>` is relative to `root`. Findings are sorted by path, then line, then rule. With `--json`, a bare JSON array of objects with `path`, `line`, `severity`, `message` and `rule`; `line` is omitted where unknown.
