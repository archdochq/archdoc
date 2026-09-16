---
id: RFC-0001
title: Guidance for coding agents
status: accepted
created: 2026-09-13
decided: 2026-09-13
backfilled: 2026-09-16
depends: []
updates: []
obsoletes: []
---

# RFC-0001: Guidance for coding agents

## Abstract

A set of guides that tell a coding agent how to work in a specification repository, written into the repository on request, refreshed by a command, and packaged as skills for hosts that discover them by name. One source serves all three, and a flag lets any command run against a repository that is not an ancestor of the working directory.

## Motivation

An agent dropped into a specification repository does the wrong thing by default: it creates a numbered file by hand, edits a frozen document to fix a typo, edits the generated index, and, when asked to backfill, writes a fluent rationale the record does not contain. Lint catches the first three. Nothing catches the fourth, or an argument written into a spec page, or an RFC where an ADR was wanted. The guidance for those has to be somewhere every agent looks, and a repository is the only such place.

The people using a repository will not all use one agent. A guide format that only one host discovers reaches only that host's users.

An agent documenting a project works from the code repository, where the evidence for backfilling lives, and writes across into the specification repository beside it. The commands locate their configuration by walking up, so from a sibling they find nothing.

## Proposal

### Concepts

| Concept | Meaning |
|---|---|
| Guide | One Markdown file addressing one task, with `name` and `description` front matter |
| Index | `AGENTS.md`, the file an agent reads first, pointing at the guides |
| Skill | A guide as a host discovers it: a directory named for the skill holding `SKILL.md` |
| Marketplace | The catalogue at a repository's root that makes its plugin installable |

### Components

| Component | Responsibility |
|---|---|
| `internal/template/agents/` | The guides, embedded verbatim |
| `internal/template/AGENTS.md` | The index, embedded verbatim |
| `archdoc init --agents` | Writes the index and the guides into a new repository |
| `archdoc agents` | Installs or refreshes them in an existing one |
| `plugin/` | The guides as skills, generated from the embedded copies |
| `.claude-plugin/marketplace.json` | The catalogue naming the plugin |
| `-C`, `--chdir` | Runs any command as if started in another directory |

### The guides

| Guide | Skill name | Covers |
|---|---|---|
| `working.md` | `archdoc-working` | The rules not to break, the commands, and which guide to read |
| `setup.md` | `archdoc-setup` | Installing the binary, locating or creating the repository, the two-repository arrangement |
| `survey.md` | `archdoc-survey` | Mining a codebase and its history for what to document, and the evidence for each |
| `classifying.md` | `archdoc-classifying` | RFC against ADR against spec page against ref |
| `authoring.md` | `archdoc-authoring` | Creating, filling and transitioning a document |
| `backfilling.md` | `archdoc-backfill` | Recording earlier decisions without inventing |
| `contributing.md` | `archdoc-contributing` | Submitting a document by pull request |
| `triage.md` | `archdoc-triage` | Reading lint output and choosing the document to edit |
| `spec-pages.md` | `archdoc-spec-pages` | Describing without arguing, `includes`, the glossary |

Every guide cites `PROCESS.md` by section name and never by line number. Each carries front matter of the form:

```yaml
---
name: archdoc-triage
description: Use when archdoc lint reports findings ...
---
```

### The index

`AGENTS.md` names the repository as managed by ArchDoc, says to read `agents/working.md` before changing anything, states that `PROCESS.md` is authoritative, and says that the file belongs to the project. It is created once and never rewritten.

### Ownership

| Path | Owner | Written by `init --agents` | Touched by `agents` |
|---|---|---|---|
| `AGENTS.md` | the repository | yes | only if absent |
| `agents/*.md` | ArchDoc | yes | replaced |

### `archdoc agents`

1. Locate the repository.
2. For each embedded guide, write it under `agents/`, creating the directory if absent, and report `written` or `updated`.
3. If `AGENTS.md` is absent, write it and report `written`; otherwise report `kept`.

Output:

```
agents/authoring.md          updated
agents/backfilling.md        updated
AGENTS.md                    kept; it is yours to edit
```

### `-C`

A persistent flag applied before any command reads the working directory. `archdoc -C ../spec lint` runs `lint` as if started in `../spec`. The directory must exist.

### Packaging

`plugin/skills/<name>/SKILL.md` is a byte-identical copy of the guide whose front matter names `<name>`, generated with `go test ./internal/template -update-plugin`. A test compares every packaged skill against its embedded source and reports a skill nothing generates. `plugin/.claude-plugin/plugin.json` describes the plugin; `.claude-plugin/marketplace.json` at the repository root catalogues it with `source: ./plugin`, and a test resolves that path and checks the two names agree.

### Errors

| Condition | Result |
|---|---|
| `-C` names a directory that does not exist | usage error naming the flag |
| `agents` run outside a repository | the usual not-found error |
| A guide cites a `PROCESS.md` section that does not exist | test failure |
| A packaged skill differs from its guide | test failure |

### Out of scope

- Guidance for contributing to ArchDoc itself. A different audience, and not shipped into repositories.
- Refreshing `PROCESS.md` or the workflow. A separate design.

## Alternatives considered

- **Skills only, with no `AGENTS.md`.** A skill reaches only a host that discovers skills by name. `AGENTS.md` is a single file at a known path that a wide range of agents read with no configuration, and it is the only place every agent looks.
- **The guidance written into `AGENTS.md` itself.** Refreshing it would overwrite whatever instructions a project had added. Keeping the content under `agents/` behind a thin pointer lets ArchDoc own one and the project the other.
- **Shipping the guides only through `init`.** A repository would carry the guides as they were at scaffold time, with no way to take a later version.

No other alternatives were weighed.

## Backwards compatibility

Nothing breaks. `--agents` is off by default, `agents` and `-C` are new, and a repository scaffolded without the flag is unchanged.

## Open questions

## Changelog

## Sources

- [7a2e4a9] adds the guides, `--agents` and `archdoc agents`, and sets `created` and `decided`.
- [462dc0d] adds `-C`, the same day.
- [89d1085] packages the guides as skills, and [d121629] adds the marketplace manifest, the same day.
- `docs/ARCHDOC.md` at [d121629a], sections Templates and `archdoc agents`, describes the arrangement.
- The alternatives were weighed in a design session on 2026-09-13, not publicly available, and first written down on 2026-09-16.

[7a2e4a9]: https://github.com/ollieread/archdoc/commit/7a2e4a9eb242c112cd673aef588f5969c40b6074
[462dc0d]: https://github.com/ollieread/archdoc/commit/462dc0d9a7e1a1fb65e891ca35128109618ef4a2
[89d1085]: https://github.com/ollieread/archdoc/commit/89d1085b229b68308737381baed0cb1b8a85689b
[d121629]: https://github.com/ollieread/archdoc/commit/d1216299169c513f7b246195012fcc39d664135f
[d121629a]: https://github.com/ollieread/archdoc/blob/d1216299169c513f7b246195012fcc39d664135f/docs/ARCHDOC.md
