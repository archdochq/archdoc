---
title: Links and the glossary
includes: []
---

# Links and the glossary

## Wiki links

`[[X]]` in a body marks a link to be filled in. `archdoc link` replaces each one in every document that is not frozen with a Markdown link:

- If `X` is an identifier: `[X: <title>](<relative path>)`.
- If `X` matches a glossary term, case-insensitively: `[X](<relative path to glossary>#<anchor>)`, keeping the case as written.
- If `X` matches both, the glossary wins.
- If `X` matches neither: error, and the document is left unchanged.

A document is either rewritten in full or not at all. A frozen document containing `[[...]]` produces a warning naming it and is never modified. A document that is terminal in the working tree and not yet on the branch is not frozen and is rewritten like any other, which is how a backfilled document's links resolve. Outside a git repository, where freezing cannot be established, a terminal document is reported as an error and left alone. Findings from `link` follow `lint`'s exit codes.

## Suggestions

`archdoc link --suggest` scans documents that have not reached a terminal status for bare identifiers and glossary terms not already inside a link or code span, and proposes linking the first occurrence of each distinct target per document. It excludes the document's own identifier, every heading, front matter, code blocks, bare URLs, raw HTML, link reference definitions, and the glossary page, which is skipped whole. A line whose brackets are unbalanced after masking is excluded whole.

A suggestion wraps the matched text exactly as written: `RFC-0007` becomes `[RFC-0007](<relative path>)`. Identifiers match their canonical uppercase form. Glossary terms match whole, case-insensitively, longest first, with no stemming and no plural handling. Whole is judged from the characters on either side of the match. A term reaching a link label is escaped.

With a terminal on stdin and no `--apply`: for each suggestion, the file, line and highlighted match are shown with the prompt `[y]es [n]o [a]ll in file [s]kip file [q]uit`; `q` writes the accepted changes and exits. The file is re-read before it is written, and left alone if it changed meanwhile. Without a terminal and without `--apply`: suggestions are printed and the exit code is 2 if there are any. With `--apply`: every suggestion is applied.

Neither `link` nor `link --suggest` rewrites a heading.

## The glossary

`spec/glossary.md` is a spec page with additional structure. Anything between the H1 and the first H2 is preamble. From the first H2 onwards, each H2 is one term followed by exactly one paragraph. Terms are unique and in ascending case-insensitive alphabetical order. A renamed term's paragraph may be followed by lines of the form `Formerly *Old Term*.`, one per line, which are not paragraphs; each rename adds a line. A glossary with no entries is valid. The file need not exist; L15 and glossary link resolution are skipped when it is absent.

`archdoc term` manages it:

- `add <term> <definition> [--from <id>...]`: inserts alphabetically, creating the page from the template if absent. Fails on a duplicate. Each `--from` identifier is appended to the page's `includes` if not present.
- `rename <old> <new> [--from <id>...]`: retitles the entry, adds `Formerly *<old>*.` after its paragraph, and re-sorts. Links elsewhere are not rewritten; L17 reports them. `link` does not repair them, since it never edits an existing Markdown link and a former name is not a term.
- `remove <term> [--from <id>...]`: deletes the entry.
- `list`: prints the terms, one per line.
- `show <term>`: prints the entry.

Each writer re-reads the page after writing it and refuses if the terms on it are not exactly those intended. A writer refuses a page whose parse stops short of the end of the file.
