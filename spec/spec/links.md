---
title: Links and the glossary
includes: []
---

# Links and the glossary

## Wiki links

`[[X]]` in a body marks a link to be filled in. `archdoc link` replaces each one in every document that is not frozen with a Markdown link:

- If `X` is an identifier: `[X: <title>](<relative path>)`.
- If `X` matches a term, case-insensitively: `[X](<relative path to GLOSSARY.md>#<anchor>)`, keeping the case as written.
- If `X` matches both, the term wins.
- If `X` matches neither: error, and the document is left unchanged.

A document is either rewritten in full or not at all. A frozen document containing `[[...]]` produces a warning naming it and is never modified. A document that is terminal in the working tree and not yet on the branch is not frozen and is rewritten like any other, which is how a backfilled document's links resolve. Outside a git repository, where freezing cannot be established, a terminal document is reported as an error and left alone. Findings from `link` follow `lint`'s exit codes.

## Suggestions

`archdoc link --suggest` scans documents that have not reached a terminal status for bare identifiers and glossary terms not already inside a link or code span, and proposes linking the first occurrence of each distinct target per document. It excludes the document's own identifier, every heading, front matter, code blocks, bare URLs, raw HTML, link reference definitions, and the glossary page, which is skipped whole. A line whose brackets are unbalanced after masking is excluded whole.

A suggestion wraps the matched text exactly as written: `RFC-0007` becomes `[RFC-0007](<relative path>)`. Identifiers match their canonical uppercase form. Glossary terms match whole, case-insensitively, longest first, with no stemming and no plural handling. Whole is judged from the characters on either side of the match. A term reaching a link label is escaped.

With a terminal on stdin and no `--apply`: for each suggestion, the file, line and highlighted match are shown with the prompt `[y]es [n]o [a]ll in file [s]kip file [q]uit`; `q` writes the accepted changes and exits. The file is re-read before it is written, and left alone if it changed meanwhile. Without a terminal and without `--apply`: suggestions are printed and the exit code is 2 if there are any. With `--apply`: every suggestion is applied.

Neither `link` nor `link --suggest` rewrites a heading.

## The glossary

A term is one file under `term/`, carrying `title`, `formerly` and `named_by` in its front matter and exactly one paragraph of definition under its H1. The term is the `title`; the filename is its slug, so two terms cannot share one. `named_by` records the document that introduced the term and is not an inclusion.

`GLOSSARY.md` is generated from `term/` and written under `root`, beside `INDEX.md`. Terms appear in ascending case-insensitive order of title. Every name in a term's `formerly` is written as an explicit anchor before its heading, so a link made before a rename still resolves; L17 checks that, and anchors are read from explicit `<a id="...">` as well as from headings for this reason.

A repository with no `term/` directory has no glossary, which is valid: L15 and term link resolution are skipped.

`archdoc glossary` regenerates the page and `--check` fails when it is out of date, as `archdoc index` does for `INDEX.md`.

`archdoc term` manages the files:

- `add <term> <definition> [--named-by <id>]`: writes `term/<slug>.md`. Refuses a slug already taken, which covers a name differing only in case, and refuses a file the index cannot see.
- `rename <old> <new>`: renames the file, sets `title`, and appends the old name to `formerly`. Links elsewhere are not rewritten and do not need to be: the old name keeps an anchor on the generated page.
- `remove <term>`: deletes the file. Refuses when a frozen document links to the term, whose link could never be corrected.
- `list`: prints the terms, one per line.
- `show <term>`: prints the term and its definition.

Each writer re-reads the page after writing it and refuses if the terms on it are not exactly those intended. A writer refuses a page whose parse stops short of the end of the file.
