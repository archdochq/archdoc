# ArchDoc skills

Guidance for agents documenting a project with [ArchDoc](https://github.com/archdochq/archdoc).

These skills are for working **from the code repository** and writing across into the
specification repository, which is where the evidence for backfilling lives.

## Installing

| Agent | Install |
| --- | --- |
| Claude Code | `/plugin marketplace add archdochq/skills`, then `/plugin install archdoc@archdoc` |
| Copilot CLI | `copilot plugin install archdochq/skills` |
| Codex | `codex plugin marketplace add archdochq/skills`, then `codex plugin add archdoc@archdoc` |
| Cursor | From the Cursor CLI. The app's import from GitHub finds no plugin at a repository root |
| Gemini CLI | `gemini extensions install https://github.com/archdochq/skills` |
| Any agent `npx skills` supports | `npx skills add archdochq/skills` |

Each release of ArchDoc is tagged here with its own tag. To stay on one, add the marketplace as
`archdochq/skills#v0.6.0` in Claude Code, or pass `--ref v0.6.0` to Gemini CLI.

## Skills

| Skill | Use when |
| --- | --- |
| `archdoc-setup` | Starting out: installing archdoc, finding or creating the spec repository |
| `archdoc-survey` | Working out what an existing project needs documented, and the evidence for it |
| `archdoc-classifying` | Deciding between an RFC, an ADR, a spec page and a ref |
| `archdoc-authoring` | Writing a document and moving it through the lifecycle |
| `archdoc-backfill` | Recording decisions taken before the repository existed |
| `archdoc-contributing` | Submitting a document by pull request to a repository you do not maintain |
| `archdoc-triage` | `archdoc lint` reported something |
| `archdoc-spec-pages` | Editing the spec, the glossary, or an `includes` list |
| `archdoc-working` | The rules that must not be broken, and which guide covers what |

## Where these come from

This repository is written by ArchDoc's release workflow from each stable release, and nothing
else writes to it. Every skill is a guide embedded in the archdoc binary of that release, the
same file `archdoc init --agents` writes into a specification repository and `archdoc agents`
refreshes, so an installed copy describes the binary that release published.

Do not open pull requests here. Change `internal/template/agents/` or `plugin/` in
[archdochq/archdoc](https://github.com/archdochq/archdoc), and the next release publishes it.

`PROCESS.md`, in the repository being documented, is authoritative wherever anything here
disagrees with it.
