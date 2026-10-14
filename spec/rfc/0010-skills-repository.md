---
id: RFC-0010
title: The guides are published from a repository of their own
status: accepted
created: 2026-10-14
decided: 2026-10-14
depends: []
updates: [RFC-0001, RFC-0008]
obsoletes: []
---

# RFC-0010: The guides are published from a repository of their own

## Abstract

The guides packaged as skills move out of this repository into `archdochq/skills`, which the release workflow writes from each stable release, with the manifests that let Claude Code, GitHub Copilot CLI, Codex, Cursor and Gemini CLI install it from git. The repository changes only when a stable release is published, and every manifest but the marketplace carries that release's version, so an installed copy holds the guides of the binary that release published. This replaces the Packaging section of [RFC-0001](0001-guidance-for-coding-agents.md), its `plugin/` and `.claude-plugin/marketplace.json` components and its error for a packaged skill that differs from its guide, and ends the regeneration of the plugin's skills that [RFC-0008](0008-questions.md) names.

## Motivation

`plugin/.claude-plugin/plugin.json` declared version `0.1.0` from v0.1.0-alpha.2, the first release to carry it, through v0.5.0. The guides changed between v0.1.0 and v0.2.0, when the repository moved to the `archdochq` organisation, and again between v0.3.0 and v0.4.0, when the module path moved, and none of the five releases from v0.2.0 to v0.5.0 raised the string. Claude Code treats a declared version as the plugin's identity for updates, and keeps every user on the copy cached at install until the string changes, however many commits land after it, so no installed copy received either change until the string was raised to `0.2.0` by hand, a change released in v0.5.1. Nothing fails when the string is left behind.

Without a declared version, Claude Code versions a plugin by the commit it was installed from, and an installed copy follows the default branch whenever it is updated. The default branch runs ahead of the latest release: a guide describing a command lands with the command, before any release carries it. The deliveries written by the binary, `init --agents`, `archdoc agents` and `archdoc update`, hold the guides of that binary's release. The plugin would be the one delivery describing a tool its user cannot install yet.

Claude Code is no longer the only agent that installs skills from a git repository. Copilot CLI, Codex and Gemini CLI each install a plugin or extension from one, Cursor's changelogs describe doing so, and `npx skills` installs bare skills into many more. All of them read skills from `skills/<name>/SKILL.md`, the layout `plugin/skills/` already has, and each agent that installs a plugin or extension finds a manifest at a path it reads, some shared between agents and some not, so one repository can carry the manifests for all of them.

## Proposal

### Concepts

| Concept | Meaning |
|---|---|
| Skills repository | `archdochq/skills`, holding the guides as skills and the manifests. Written only by the release workflow |
| Skeleton | Everything the skills repository carries except the skills, the versions and the licence, kept in this repository under `plugin/` |
| Manifest | A file through which an agent finds or versions the plugin |
| Generator | `plugin/build.go`, which writes the skills repository's contents from a checkout of this repository |

### Components

| Component | Responsibility |
|---|---|
| `internal/template/agents/` | The guides, unchanged, and the only source of every skill |
| `plugin/` | The skeleton: every manifest and the skills repository's `README.md`, beside the generator. `plugin/skills/` is removed |
| `plugin/build.go` | The generator |
| `plugin/README.md` | Rewritten as the skills repository's README: what the repository is, the install line for each agent, and that the release workflow writes it |
| `.claude-plugin/marketplace.json` | Removed from this repository's root; the marketplace moves to the skills repository |
| `README.md` | The sentence that the guides are packaged under `plugin/`, the install lines, and the account of the deliveries that says the packaged copies are compared byte for byte, all replaced by the skills repository and its install line |
| The release workflow | A `skills` job that runs the generator from the tag and pushes the result |
| `internal/template` tests | Check the skeleton, the guides' skill names and the generator's output, in place of comparing a generated copy |
| `archdochq/skills` | The published plugin, tagged with each release it carries |
| Spec pages | `layout.md` for the packaging, `architecture.md` for the tree, `release.md` for the job |

### The skills repository

```
archdochq/skills
├── .claude-plugin/marketplace.json    marketplace: Claude Code, Codex
├── .claude-plugin/plugin.json         Claude Code manifest
├── .cursor-plugin/plugin.json         Cursor manifest
├── gemini-extension.json              Gemini CLI manifest
├── plugin.json                        Agent Plugins 1.0 manifest: Copilot CLI, Codex
├── skills/<name>/SKILL.md             one per guide
├── LICENSE
└── README.md
```

The plugin is the repository root. The marketplace lists one plugin, `archdoc`, whose source is the root itself, `"."`. Codex finds neither `.agents/plugins/marketplace.json` nor `.agents/plugins/api_marketplace.json` and reads `.claude-plugin/marketplace.json`, accepting a plain path as a source, `.` as the marketplace root, and an entry with no `policy`. OpenAI's plugin guide says to start a source with `./` and to include `policy` and `category` on every entry; the Codex source accepts `.` and defaults `policy`.

| Agent | Reads | Install |
|---|---|---|
| Claude Code | `.claude-plugin/marketplace.json`, `.claude-plugin/plugin.json` | `/plugin marketplace add archdochq/skills`, then `/plugin install archdoc@archdoc` |
| Copilot CLI | `plugin.json` | `copilot plugin install archdochq/skills` |
| Codex | `.claude-plugin/marketplace.json`, `plugin.json` | `codex plugin marketplace add archdochq/skills`, then `codex plugin add archdoc@archdoc` |
| Cursor | `.cursor-plugin/plugin.json` or `plugin.json` | From the Cursor CLI, which installs a plugin at the root of a GitHub repository. The app's import from GitHub finds no plugin at a repository root |
| Gemini CLI | `gemini-extension.json`, `skills/` | `gemini extensions install https://github.com/archdochq/skills` |
| Any agent `npx skills` supports | `skills/` | `npx skills add archdochq/skills` |

Copilot CLI reads the root `plugin.json` in preference to `.claude-plugin/plugin.json` when both exist, and Claude Code reads only the latter, so the two manifests coexist. Copilot CLI installs from the root of a GitHub repository directly, without the marketplace. Cursor loads a plugin from either `.cursor-plugin/plugin.json` or a root `plugin.json` following the Agent Plugins specification, and which it uses when both are present is not documented.

### Manifests

Every manifest names the plugin `archdoc`, and every description the manifests carry is the same. `.claude-plugin/plugin.json`, `.cursor-plugin/plugin.json` and `plugin.json` also carry `author`, holding `name` alone, `homepage`, `repository`, and `license` as `AGPL-3.0-or-later`, the licence [ADR-0012](../adr/0012-the-licence-is-agpl-3-0-or-later.md) chose. Gemini CLI's manifest defines none of those four fields. The marketplace is named `archdoc`, so the Claude Code and Codex install argument is `archdoc@archdoc`.

| Manifest | Carries a version | Notes |
|---|---|---|
| `.claude-plugin/plugin.json` | yes | Claude Code versions the plugin by it |
| `.cursor-plugin/plugin.json` | yes | The manifest Cursor's plugin template keeps at the root of a repository holding one plugin |
| `plugin.json` | yes | `$schema` is `https://agent-plugins.org/schemas/1.0.0/plugin.schema.json` |
| `gemini-extension.json` | yes | No `contextFileName`; the guides are skills, not context |
| `.claude-plugin/marketplace.json` | no | A version on a Claude Code marketplace entry is overridden by `plugin.json`, and reported by `claude plugin validate` when the two differ |

The skeleton's manifests carry no `version`. The generator adds it.

Claude Code decides an installed copy is out of date by its version. Codex compares a git marketplace's revision against the one it last installed, and reinstalls its plugins when the two differ. Gemini CLI installs a repository with no GitHub release by cloning it, and compares the installed commit against the default branch; the skills repository publishes no GitHub release. Copilot CLI and Cursor do not document how they decide. The release workflow is the only writer to the skills repository, and it writes once per stable release with a new version, so each of these signals changes exactly when a release is published.

### `plugin/build.go`

`plugin/build.go` is a `main` package excluded from every build by a `//go:build ignore` constraint, so `go build ./...` and `go install ./...` never produce it. It is run from the repository root as `go run ./plugin/build.go <directory> <version>`, uses the standard library and the guides embedded by `internal/template`, and writes into `<directory>`.

1. Exit non-zero without writing anything unless there are exactly two arguments, `<version>` is not empty, `<directory>` exists, and every guide declares a `name` in its front matter that follows the Agent Skills rules the tests check and that no other guide declares.
2. Remove everything in `<directory>` except `.git`.
3. Copy the skeleton, `plugin/` without `build.go`, into it.
4. Copy each guide byte for byte to `skills/<name>/SKILL.md`.
5. Copy `LICENSE` from the repository root.
6. Re-encode `.claude-plugin/plugin.json`, `.cursor-plugin/plugin.json`, `plugin.json` and `gemini-extension.json` with `version` set to `<version>`.

Emptying the directory first removes a skill whose guide was removed, and a manifest the skeleton no longer carries. The generator commits nothing. Run from a clean checkout of a tag, it writes what that release publishes; run from a working tree, it copies whatever that tree holds under `plugin/`, untracked files included.

### The `skills` job

The job runs in the release workflow after the release is published, for a tag carrying no prerelease identifier, matching the Homebrew tap. A prerelease publishes nothing.

1. Check out this repository at the tag, and `archdochq/skills` at its default branch using the `SKILLS_DEPLOY_KEY` secret, a deploy key with write access to that repository alone.
2. Set up Go, and run the generator with the tag less its leading `v`, against the skills repository's checkout.
3. Commit as `github-actions[bot]` with the message `feat(archdoc): Release <tag>`, tag the commit `<tag>` with a lightweight tag, and push both in one atomic push, so either both land or neither does.

`archdochq/skills` is created with one commit holding a README, so its default branch exists before the job first runs. The skills repository's tag lets a user of Claude Code or Codex pin a release through the marketplace's ref, and a user of Gemini CLI through `--ref`, which asks before installing a ref that has no GitHub release. The tag is lightweight because Gemini CLI checks a pinned install for updates by comparing the commit it holds with the hash `git ls-remote` gives for the ref, which for an annotated tag is the tag object's. Copilot CLI's install from a repository root takes no ref.

### Tests

| Test | Checks |
|---|---|
| `TestEveryGuideIsAlsoAValidSkill`, extended | Each guide's front matter holds only keys the Agent Skills specification defines. Its `name` is 1 to 64 characters from `a` to `z`, `0` to `9` and `-`, neither starting nor ending with a hyphen and with no two in a row, and unique across guides. Its `description` is 1 to 1024 characters and not whitespace alone. The existing checks for both fields and the `archdoc-` prefix remain |
| The skeleton holds the tree above | Exactly the manifests and `README.md` the tree names, besides `build.go`; each manifest valid JSON naming the plugin `archdoc`, and none declaring a version |
| The marketplace names the root as its plugin's source | `"."` |
| The generator, run with `go run` from the repository root into a temporary directory | One `SKILL.md` per guide, in a directory named for the guide's `name` and identical to it, and the given version in each versioned manifest |

`TestThePluginSkillsMatchTheShippedGuides`, `TestThePluginManifestIsPresentAndNamesTheRightSkills` and `TestTheMarketplaceManifestResolves` are removed, since each reads `plugin/skills/` or the root marketplace, as is the `-update-plugin` flag.

### Errors

| Condition | Result |
|---|---|
| The generator's arguments are missing or empty, or `<directory>` does not exist | The generator exits non-zero before writing anything |
| A guide declares no `name`, one outside the Agent Skills rules, or one another guide declares | The tests fail, and the release workflow's test step stops the release before it is published. The generator, run outside a release, exits non-zero before writing anything |
| `SKILLS_DEPLOY_KEY` is unset | The checkout falls back to the workflow's own token, which cannot push to another repository, so the push fails. The release itself is already published |
| `archdochq/skills` does not exist | The checkout fails. The release itself is already published |
| The push to `archdochq/skills` is rejected | The job fails with neither the commit nor the tag pushed, and can be re-run against the same tag |
| A guide's `description` breaks the Agent Skills rules | The tests fail, and the release workflow's test step stops the release before it is published. The generator does not check it |

## Alternatives considered

- **Leaving `version` out of the manifest.** Claude Code would version the plugin by commit, and every change would reach users on update, but from the default branch, ahead of the binary they can install.
- **Bumping `version` by hand at each release.** What this replaces. `0.1.0` went unraised through five releases without anything failing.
- **Keeping the marketplace in this repository with a `github` source naming the skills repository.** Existing installs would keep resolving, but a marketplace and its test would remain here, and the manifests for the other agents would sit in a different repository from Claude Code's.
- **Copying the guides and writing the versions in the workflow's own steps.** Nothing outside a release run could produce the published contents. A program can be run locally against a tag, and tested.
- **A shell script using `jq`.** `jq` would be a build-time dependency beside goreleaser, the only one [ADR-0005](../adr/0005-the-dependency-list-is-closed.md) admits.
- **A generator behind a test flag, as `-update-plugin` is.** The repository's test-flag generators write files committed here and compared by the same test. This output is committed elsewhere, and is a build step run by the release.
- **A command under `cmd/`.** `go install ./...`, which the pre-commit hooks run, would build and install it beside `archdoc`.
- **The plugin in a subdirectory of the skills repository.** Gemini CLI reads its manifest from the repository root, Copilot CLI installs from the root, and Claude Code and Codex accept the root as a marketplace source.
- **A version on the marketplace entry.** Claude Code reads the version from `plugin.json` over the entry's, so a second copy could only differ from the first.
- **Cursor reading the root `plugin.json` alone.** Cursor's plugin template keeps `.cursor-plugin/plugin.json` at the root of a repository holding one plugin, and Cursor does not say which manifest it uses when both are present.
- **A marketplace file of its own for Codex.** Codex reads the Claude Code marketplace when it finds none of its own.
- **A marketplace file for Cursor.** Its one documented use is the app's import from a GitHub repository, which indexes no plugin at a repository root with one or without.

No other alternatives were weighed.

## Backwards compatibility

A marketplace added as `archdochq/archdoc` with no ref, or as `ollieread/archdoc` before the repository moved, stops resolving once `.claude-plugin/marketplace.json` leaves this repository's default branch. An installed copy receives nothing further from it. One added with a ref naming an earlier tag keeps resolving to that tag's copy. Moving takes `/plugin marketplace remove archdoc`, `/plugin marketplace add archdochq/skills` and `/plugin install archdoc@archdoc`, in that order: one user can register only one marketplace of a given name, and removing a marketplace uninstalls the plugins installed from it. Generated release notes list commits by subject, so the three commands are added by hand to the notes of the first release the job publishes.

The removal lands as the last change before the tag of the first release that runs the job, and that run is the skills repository's first write after its initial commit.

`go test ./internal/template -update-plugin` no longer exists, so the regeneration of the plugin's skills that [RFC-0008](0008-questions.md) names among its components no longer happens. The README's account of the plugin and its install lines change.

## Open questions

## Resolved questions

### Does Codex accept the marketplace root as the source path of a plugin?

OpenAI's guide requires a path that starts with `./` and stays inside the marketplace root, and shows only subdirectories. If the root is refused, the plugin moves to a subdirectory for every agent, since Gemini CLI reads its manifest from the repository root and Copilot CLI installs from it.

> Yes. Codex resolves a local source of `.` or `./` to the marketplace root. It reads the first of `.agents/plugins/marketplace.json`, `.agents/plugins/api_marketplace.json`, `.claude-plugin/marketplace.json` and `.cursor-plugin/marketplace.json` that exists, accepts a plain path as a source, and defaults an absent `policy`, so the skeleton carries no Codex marketplace of its own.

### How does a Cursor user install from a repository that is not listed in the Cursor Marketplace?

Cursor's plugin reference documents `.cursor-plugin/marketplace.json` and submission to the Cursor Marketplace, but not adding a third-party marketplace. If only listed plugins can be installed, the Cursor marketplace file serves nothing and the skeleton drops it.

> ~~`/add-plugin` with a repository URL, typed in the chat input, parses the repository and installs its plugin. Cursor's plugin template says a repository holding one plugin keeps it at the root with no `.cursor-plugin/marketplace.json`, so the skeleton carries none, and Cursor reads the root `plugin.json`. Importing a repository as a personal marketplace from the Customize page indexes no plugins in Cursor 3.22 and 3.23, so the install line names `/add-plugin` instead.~~ Struck on 2026-10-14: Cursor's skills documentation requires `.cursor-plugin/marketplace.json` for an import from a GitHub repository, which accounts for the import indexing no plugins, and no route has been tried against this layout. The question continues as [Does Cursor's import from a GitHub repository accept a marketplace whose plugin source is the root?](#does-cursors-import-from-a-github-repository-accept-a-marketplace-whose-plugin-source-is-the-root)

### Does Cursor's import from a GitHub repository accept a marketplace whose plugin source is the root?

Cursor's documentation installs skills from a GitHub repository by importing it in Customize with From GitHub Repository, which requires `.cursor-plugin/marketplace.json`, or by adding it as a team marketplace. Its plugin reference shows a marketplace's plugins in subdirectories, and does not say whether a source of `"."` is accepted. Its changelogs describe further routes: `/add-plugin` in the editor, a repository URL pasted into the CLI's plugin search, a marketplace added by git URL in the CLI, and the import of plugins installed in Claude Code. None has been tried against this layout.

> No. The Cursor app's import from a GitHub repository adds the repository as a marketplace and indexes no plugin from it when the plugin is the repository root, whether `.cursor-plugin/marketplace.json` lists it with a source of `./` or the repository carries no marketplace file. The marketplaces Cursor's examples link list their plugins in subdirectories. The Cursor CLI installs a plugin at the root of a GitHub repository. The install line names the CLI, and the skeleton carries no Cursor marketplace, whose one documented use is that import.

## Changelog

- 2026-10-14: Both open questions resolved. The Codex and Cursor marketplace files are dropped from the skills repository; Codex reads the Claude Code marketplace, and Cursor installs the root plugin with `/add-plugin`.
- 2026-10-14: The Cursor question reopened, and `.cursor-plugin/plugin.json` added to the skills repository. Copilot CLI installs from the repository root rather than through the marketplace. The Motivation corrected: the version left unraised through five releases was `0.1.0`.
- 2026-10-14: The generator is `plugin/build.go`, run with `go run`, in place of a shell script using `jq`. The Codex question moves to `Resolved questions`, and the document updates [RFC-0008](0008-questions.md).
- 2026-10-14: `.cursor-plugin/marketplace.json` returns, since Cursor's documented import from a GitHub repository requires it, and the Cursor question narrows to whether that import accepts the root as a source. Codex is described as following the marketplace's revision rather than the version. The generator checks names against the Agent Skills rules, and the job pushes the commit and the tag atomically.
- 2026-10-14: The skills repository's tags are lightweight, for Gemini CLI's update check. The resolved Codex question names the root rather than `./`, and a marketplace pinned to an earlier tag is noted as still resolving.
- 2026-10-14: The first Cursor question and its struck answer move to `Resolved questions`.
- 2026-10-14: The Cursor question is resolved: the app's import indexes no plugin at a repository root, with a marketplace file or without one, and the Cursor CLI installs one. Cursor's install line names the CLI, and `.cursor-plugin/marketplace.json` leaves the skills repository.
