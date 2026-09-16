---
id: RFC-0003
title: Refreshing generated files
status: accepted
created: 2026-09-13
decided: 2026-09-13
backfilled: 2026-09-16
depends: []
updates: []
obsoletes: []
---

# RFC-0003: Refreshing generated files

## Abstract

A command that regenerates the files ArchDoc writes and a repository then keeps, shows what would change, and asks before writing. It leaves the version the workflow pins as it is.

## Motivation

`PROCESS.md`, the workflow and the agent guides ship with the binary and change as ArchDoc changes. A repository scaffolded by an older version carries older copies, including any that were wrong. A defect in the tool that put a wrong file into every repository it scaffolded left each of those needing the same edit by hand, with nothing to make it.

## Proposal

### Concepts

| Concept | Meaning |
|---|---|
| Generated file | A file `init` writes from the binary that the repository then owns a copy of |
| Refresh | Replacing that copy with what the running binary would write now |

### Components

| Component | Responsibility |
|---|---|
| `archdoc update` | Plans every refresh, diffs, asks, writes |
| `unifiedDiff` | Renders the change to each file |

### What is refreshed

| File | Refreshed | Created if absent |
|---|---|---|
| `PROCESS.md` | yes | yes |
| `.github/workflows/archdoc-lint.yml` | yes | yes |
| `agents/*.md` | yes, where `agents/` exists | no |

### What is never touched

`AGENTS.md`, `README.md`, `archdoc.json`, `LICENSE`, `INDEX.md`, and every document. Where `agents/` is absent the command says that `archdoc agents` installs it.

### Steps

1. Locate the repository.
2. For each file refreshed, render what the binary would write and read what is on disk.
3. Print a unified diff for each file that differs, then the notes about what was left alone.
4. If nothing differs, say so and exit 0.
5. With `--check`, exit 2.
6. Without `--yes`: if there is no terminal, refuse, naming `--yes`; otherwise ask, and stop on no.
7. Write each differing file and print its path.

### The version pin

The workflow's `ARCHDOC_VERSION` is read from the existing workflow and written back unchanged. The running binary's version is used only where there is no existing pin to read.

### Output

```
--- .github/workflows/archdoc-lint.yml
+++ .github/workflows/archdoc-lint.yml (generated)
@@ -58,7 +58,6 @@
-        working-directory: ../../../code/thegamepanel/spec
         run: archdoc lint
AGENTS.md, README.md and archdoc.json are yours and were not read
Overwrite 1 file(s) with the versions above? [y/N]
```

### Errors

| Condition | Result |
|---|---|
| No terminal and no `--yes` | refusal naming `--yes` and `--check`, exit 1 |
| Changes pending with `--check` | exit 2 after the diff |

### Out of scope

- Installing `agents/` where it is absent. `archdoc agents` does that.
- Refreshing `INDEX.md`. `archdoc index` does that.

## Alternatives considered

- **A confirmation prompt without a diff.** "Are you sure" cannot be answered without knowing what will change. The diff is the safety mechanism, and the prompt follows it.
- **Raising the version pin to the running binary.** The pin records which ArchDoc the repository's CI runs, and the template asks for it to be raised deliberately. A binary that is not a published release would write `latest` over a real version.

No other alternatives were weighed.

## Backwards compatibility

Nothing breaks. The command is new.

## Open questions

## Changelog

## Sources

- [f6cbcf7] adds the command and sets `created` and `decided`. Its message states that the pin is left alone and why.
- `docs/ARCHDOC.md` at [24cceb6a], section `archdoc update`, describes the command as first documented, on 2026-09-14.
- The alternatives were weighed in a design session on 2026-09-13, not publicly available, and first written down on 2026-09-16.

[f6cbcf7]: https://github.com/ollieread/archdoc/commit/f6cbcf78069d6a4b190578cb9cfb9d6edaf4a28f
[24cceb6a]: https://github.com/ollieread/archdoc/blob/24cceb60c0e5f71122b0faff4ec4bf3f60cdb605/docs/ARCHDOC.md
