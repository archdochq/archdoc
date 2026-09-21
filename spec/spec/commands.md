---
title: Commands
includes: [RFC-0001, RFC-0002, RFC-0003, RFC-0005, ADR-0016]
---

# Commands

## Locating the repository

Every command except `init` locates `archdoc.json` by walking up from the working directory and fails if none is found. `-C <dir>`, `--chdir <dir>`, is a persistent flag that runs the command as if it had been started in that directory, applied before anything reads the working directory. Every command operates relative to `root`.

Exit codes: 0 on success, 1 on a usage or validation error, 2 when lint, a check or a link scan finds errors. Nothing under `internal/` calls `os.Exit`; the status travels as an error type to the command layer.

## Interaction

With a terminal on stdin, a command prompts for every value it was not given: required arguments, and optional flags whose value is written to a file. Flags that select a mode of operation (`--json`, `--check`, `--suggest`, `--apply`, `--strict-warnings`, `--no-interaction`, `--yes`) are never prompted for. A flag supplied on the command line is not prompted for. The prompts for one invocation form a single form, required arguments first, each optional pre-filled with its default. Without a terminal, or with `--no-interaction`, defaults apply and a missing required argument is a usage error. `--no-interaction` is a persistent flag. A terminal is detected with `term.IsTerminal`.

Prompts offer a filtered choice where one exists: `new` prompts for type then title; a transition command prompts with a select of the documents eligible for that transition, showing identifier, title and status; `term add` prompts for term and definition, then a multi-select of accepted documents for `--from`; `init` prompts for each configuration value in turn.

## Output

Plain text on stdout. Findings take the form `<path>:<line>: <severity>: <message>` where a line is known and `<path>: <severity>: <message>` otherwise, with `<path>` relative to `root`. `--json` on `lint`, `index --check` and `link` emits the same findings as a bare JSON array of objects with `path`, `line`, `severity`, `message` and `rule`; `line` is omitted where unknown, and `rule` is the rule code, or `index` for the index check. Progress notices go to stderr.

## `archdoc init`

Scaffolds a repository in the current directory.

| Option | Default |
|---|---|
| `--name=<string>` | the directory name with a trailing `-spec` removed |
| `--branch=<string>` | the current git branch, or `main` |
| `--root=<path>` | `.` |
| `--strict` / `--no-strict` | strict |
| `--ref-stale-days=<int>` | 180 |
| `--license=<none\|mit>` | `none` |
| `--agents` | off |

Writes `archdoc.json` in the current directory; `README.md`, `PROCESS.md`, `INDEX.md`, `rfc/.gitkeep`, `adr/.gitkeep`, `ref/.gitkeep`, `spec/glossary.md`, `LICENSE` if requested, and `AGENTS.md` with `agents/` if requested, under `root`; and `.github/workflows/archdoc-lint.yml` at the git repository root, or in the current directory where there is no repository. Every file is planned before any is written, collisions are tested with `Lstat`, and files are created with `O_EXCL`. Prints every path written. Refuses if `archdoc.json` already exists or any other planned file is in the way. Validates the configuration through the same path that loads one, after prompting and before writing. Does not initialise git and does not commit.

The generated workflow runs on push and pull request. It checks out the full history, uses `archdochq/lint` pinned to the version that ran `init`, which installs `archdoc` and reports each finding as an annotation on the line of the document that caused it, then runs `archdoc index --check`. Its `working-directory` is the path from the git repository root to the directory holding `archdoc.json`, as git reports it, and is omitted when the two are the same.

## `archdoc new <rfc|adr|ref> <title>`

Creates the next-numbered document of the type from its template, with `ID`, `Title` and `Date` substituted, and prints the path. Refuses a title that is empty, slugs to nothing, or contains a line break, and refuses if the derived filename exists. Refuses to write a heading that does not read back as the title given.

| Option | Effect |
|---|---|
| `--backfill` | create the document already terminal. RFCs and ADRs only |
| `--status=<accepted\|rejected\|withdrawn>` | with `--backfill`, the terminal status. Default `accepted` |
| `--created=<date>`, `--decided=<date>` | with `--backfill`, the historical dates. `--decided` defaults to `--created` |

## `archdoc renumber <id|path> [new-id]`

Changes a numbered document's identifier. Rewrites the filename keeping the slug, the front matter `id`, the H1, and every reference to the old identifier: the `depends`, `updates`, `obsoletes` and `includes` lists of every other document, `[[...]]` links naming it, and link destinations pointing at the renamed file. With no target, the next free number for the type is taken.

The selector is an identifier or a path. Given an identifier carried by more than one document, the command lists the candidates and refuses. Refuses a frozen document, and refuses when any frozen document references the one being renamed. Nothing is written unless everything can be. The index is not regenerated, and the command says so.

## `archdoc update [--check] [--yes]`

Regenerates `PROCESS.md`, the workflow, and the guides under `agents/` where that directory exists. Shows a unified diff of everything that would change and asks before writing. `--check` shows and exits 2 without writing. `--yes` applies without asking. With neither and no terminal, it refuses.

`AGENTS.md`, `README.md`, `archdoc.json`, `LICENSE`, `INDEX.md` and every document are not touched. `agents/` is refreshed and never created; where it is absent the command says that `archdoc agents` installs it.

The workflow's `version` pin is raised to the version doing the update. It is kept as it is only when the running binary's version is not a published release. A repository scaffolded before the workflow used `archdochq/lint` names its pin `ARCHDOC_VERSION`, and that spelling is read too, so the pin survives the change.

## `archdoc agents`

Writes the guides under `agents/`, replacing what is there, and creates `AGENTS.md` only when it is absent. Reports, for each path, whether it was written, updated or kept. Creates `agents/` where the repository was scaffolded without it.
