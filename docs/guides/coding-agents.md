# Working with coding agents

ArchDoc ships a set of guides written for coding agents: how to classify a document, write one, backfill a decision, read lint output and contribute by pull request. You can give them to an agent in two ways, and both carry the same files.

- **In the specification repository**, as `AGENTS.md` and an `agents/` directory that `archdoc init --agents` or `archdoc agents` writes. Any agent working in that repository can read them.
- **In the agent tool**, as skills installed from [archdochq/skills](https://github.com/archdochq/skills). These are for an agent working from the code repository and writing across into the specification repository.

The guides are distinct from the pages you are reading now. These pages are written for you; the agent guides are written for an agent to follow, as instructions.

## Guides in the specification repository

### When you scaffold the repository

Pass `--agents` to `archdoc init`:

```sh
archdoc init --agents
```

Besides everything `init` always writes, this writes `AGENTS.md` and these guides under `agents/`, in the directory that holds the documents:

```text
AGENTS.md
agents/authoring.md
agents/backfilling.md
agents/classifying.md
agents/contributing.md
agents/setup.md
agents/spec-pages.md
agents/survey.md
agents/triage.md
agents/working.md
```

### Adding them to an existing repository

Run `archdoc agents`. It writes every guide under `agents/`, replacing what is there, creates `agents/` if the repository was scaffolded without it, and creates `AGENTS.md` only when it is absent. It reports what it did to each path:

```text
agents/authoring.md          written
...
agents/working.md            written
AGENTS.md                    written
```

Run again, it reports each guide as `updated` and `AGENTS.md` as `kept; it is yours to edit`.

### AGENTS.md is yours

`AGENTS.md` tells an agent to read `agents/working.md` before changing anything, and that `PROCESS.md` is authoritative wherever anything disagrees with it. ArchDoc creates it once and never rewrites it: neither `archdoc agents` nor `archdoc update` touches it. Add your project's own instructions to it.

The files under `agents/` are the opposite. They are regenerated from the binary, so an edit you make to one is lost the next time they are refreshed. Put anything specific to your project in `AGENTS.md` instead.

## What the guides tell an agent

`agents/working.md` is the entry point. It summarises the process for an agent and lists the rules it must not break:

1. Never create an RFC, ADR or ref by hand: run `archdoc new rfc|adr|ref "Title"`, because the number is the document's identity and the tool assigns it. Spec pages and groups are not numbered and are written by hand.
2. Never edit a document whose status is `accepted`, `rejected` or `withdrawn`. Write a new document that updates or obsoletes it instead, even for a typo.
3. Never edit `INDEX.md`. Run `archdoc index`.
4. Never invent anything when backfilling. Where the record does not say, write that it does not say.
5. Never argue in a spec page. Argument belongs in the RFCs and ADRs the page includes.
6. Never delete a document. Withdraw it.
7. Run `archdoc lint` before finishing. Exit code 2 means something is wrong.

It also lists the commands, and points to a guide for each task. Each guide is also published as the skill named here:

| Guide | Skill | When |
| --- | --- | --- |
| `setup.md` | `archdoc-setup` | Starting out: installing archdoc, finding or creating the spec repository |
| `survey.md` | `archdoc-survey` | Working out what an existing project needs documented, from git history and pull requests, as a plan before anything is written |
| `classifying.md` | `archdoc-classifying` | Deciding between an RFC, an ADR, a spec page and a ref |
| `authoring.md` | `archdoc-authoring` | Writing a document and moving it through the lifecycle |
| `backfilling.md` | `archdoc-backfill` | Recording decisions taken before the repository existed, with their sources |
| `contributing.md` | `archdoc-contributing` | Submitting a document by pull request to a repository the agent does not maintain |
| `triage.md` | `archdoc-triage` | `archdoc lint` reported something: each rule's remedy, which document to edit, and which findings must not be fixed |
| `spec-pages.md` | `archdoc-spec-pages` | Editing the spec, the glossary, a group, or an `includes` list |
| `working.md` | `archdoc-working` | The rules that must not be broken, and which guide covers what |

The guides are written as though the agent's working directory is inside the specification repository. An agent working from the code repository is told to add `-C` and the path to every command rather than changing directory, for example:

```sh
archdoc -C ../spec lint
archdoc -C ../spec new rfc "Retry policy"
```

[A spec repository beside the code](beside-the-code.md) covers that arrangement. The guides for backfilling and contributing correspond to [Backfilling](backfilling.md) and [Contributing by pull request](pull-requests.md), which describe the same process for you.

## Skills from archdochq/skills

Each stable release of ArchDoc writes the guides into [archdochq/skills](https://github.com/archdochq/skills) as skills, one per guide, named as in the table above. Each skill is the same file that release's binary writes into `agents/`.

Install them with your agent tool:

| Agent | Install |
| --- | --- |
| Claude Code | `/plugin marketplace add archdochq/skills`, then `/plugin install archdoc@archdoc` |
| Copilot CLI | `copilot plugin install archdochq/skills` |
| Codex | `codex plugin marketplace add archdochq/skills`, then `codex plugin add archdoc@archdoc` |
| Cursor | From the Cursor CLI. The app's import from GitHub finds no plugin at a repository root |
| Gemini CLI | `gemini extensions install https://github.com/archdochq/skills` |
| Any agent `npx skills` supports | `npx skills add archdochq/skills` |

In Claude Code, for example:

```text
/plugin marketplace add archdochq/skills
/plugin install archdoc@archdoc
```

Each release is tagged in archdochq/skills with its own tag. To stay on one release, add the marketplace as `archdochq/skills#v0.6.0` in Claude Code, or pass `--ref v0.6.0` to Gemini CLI.

archdochq/skills is written only by ArchDoc's release workflow, and does not take pull requests. A change to a guide is made in [archdochq/archdoc](https://github.com/archdochq/archdoc) and published by the next release.

## Keeping the guides current

The guides change as ArchDoc changes, so a repository scaffolded by an older version carries older guidance. After you upgrade the binary, refresh them.

`archdoc update` regenerates `PROCESS.md`, the workflow, and the guides under `agents/` where that directory exists. It shows a unified diff of everything that would change and asks before writing:

```sh
archdoc update            # show the diff and ask
archdoc update --check    # show the diff and exit 2 if anything would change, writing nothing
archdoc update --yes      # apply without asking
```

Without a terminal, when something would change, `archdoc update` shows the diff, writes nothing and exits 1 unless you pass `--yes`. It never creates `agents/`; where the directory is absent it says that `archdoc agents` installs it. It does not touch `AGENTS.md`, `README.md`, `archdoc.json`, `LICENSE` or any document. Run from a release binary, regenerating the workflow moves its version pin to that release, which the diff shows; any other binary keeps the pin.

`archdoc agents` refreshes the guides alone, without the diff or the question, and leaves `PROCESS.md` and the workflow as they are.

An installed skill holds the guides of the release that wrote it. To move to a later release's guides, update the plugin or extension through your agent tool, or change the tag you pinned.
