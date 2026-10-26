# Checking in CI: GitHub, GitLab and pre-commit

A specification repository is checked by four commands. This guide covers running them on GitHub Actions, on GitLab CI, and before each commit with pre-commit.

## The checks

| Command | What it checks |
|---|---|
| `archdoc lint` | Every L rule over every document, including that no frozen document has changed since it was frozen on the configured branch |
| `archdoc lint --merge` | The L rules, and the merge rules M01 to M04, which judge what a pull request changes in the record against the configured branch |
| `archdoc index --check` | That `INDEX.md` is what `archdoc index` would write |
| `archdoc glossary --check` | That `GLOSSARY.md` is what `archdoc glossary` would write |

The merge rules report an RFC or ADR that arrives as a draft, a document arriving with a number the branch already uses, a change of status the lifecycle does not permit from the status on the branch, and the deletion or renumbering of a document proposed on the branch.

`archdoc changes` lists what the working tree changes in the record against the configured branch, as a Markdown table, or as JSON with `--json`. It judges nothing, and exits 0 whatever it lists.

### Exit codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | A usage or validation error, or any other failure, printed to stderr |
| `2` | Lint reported an error, or a warning under `--strict-warnings`; a `--check` found a file to change; `link` reported an error, or `link --suggest` has a suggestion it cannot ask about without `--apply` |

Warnings do not fail `archdoc lint` unless you pass `--strict-warnings`.

### History

The checks read the configured branch, named by `branch` in `archdoc.json`, from git. Frozen documents are compared with that branch, and `lint --merge` and `changes` compare against the merge base of `HEAD` and that branch. Where there is no local branch of that name, `archdoc` reads `origin/<branch>`, which is what a CI checkout usually has.

A shallow clone holding only the commit under test fails these checks. Without the branch, `archdoc lint` reports:

```text
.: error: branch "main" does not exist, so no document can be checked against it
```

and `archdoc lint --merge` exits 1 with `no branch to compare against`. With the branch but too little history to find a merge base, it exits 1 saying `HEAD shares no history with main`. Every CI setup below fetches the full history and the configured branch.

## GitHub Actions

`archdoc init` writes `.github/workflows/archdoc-lint.yml` at the root of the git repository. For a repository whose configured branch is `main`, scaffolded by `v0.6.0`, it reads:

```yaml
name: archdoc lint

on:
  push:
  pull_request:

permissions:
  contents: read

jobs:
  archdoc:
    name: Check the specification
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
        with:
          fetch-depth: 0

      - uses: archdochq/lint@v1
        with:
          version: v0.6.0
          merge: ${{ github.event_name == 'pull_request' && github.base_ref == 'main' }}

      - name: Check the generated files are current
        run: |
          archdoc index --check
          archdoc glossary --check

      - name: Summarise the changes to the record
        if: ${{ !cancelled() && github.event_name == 'pull_request' && github.base_ref == 'main' }}
        run: archdoc changes >> "$GITHUB_STEP_SUMMARY"
```

The file `init` writes also carries comments explaining each step, left out here.

- It runs on every push and every pull request.
- `fetch-depth: 0` checks out the full history.
- `archdochq/lint` installs the `archdoc` release named by `version`, runs `archdoc lint`, reports each finding as an annotation on the line of the document that caused it, and leaves `archdoc` on `PATH` for the later steps.
- On a pull request into the configured branch, `merge` is true and the action runs `archdoc lint --merge` instead.
- `archdoc index --check` and `archdoc glossary --check` fail the job when either generated file is out of date.
- On a pull request into the configured branch, the last step appends the `archdoc changes` table to the run's summary page. It runs whatever the checks before it found, unless the run was cancelled.

The configured branch is written into both conditions when the workflow is generated. If you change `branch` in `archdoc.json`, run `archdoc update` to regenerate the workflow with the new branch.

Where `archdoc.json` is not at the root of the git repository, as in a repository scaffolded before `init` wrote it there, the workflow also gives the action a `path` input and each later step a `working-directory`, both naming the directory holding `archdoc.json`.

### The version pin

`version` is the release that ran `init`. A binary whose version is not a release tag, such as one built from source, writes `latest` and warns that the workflow is unpinned. The pin does not move by itself, so the rules a repository is held to change only when you raise it.

### Keeping the workflow current

After upgrading `archdoc`, regenerate the files it owns:

```sh
archdoc update
```

`update` regenerates `PROCESS.md`, the workflow, and the guides under `agents/` where that directory exists. It shows a unified diff of everything that would change and asks before writing. Run from a release binary, regenerating the workflow sets `version` to that release; any other binary keeps the pin the workflow has. `--check` shows the diff and exits 2 without writing where anything would change, and `--yes` applies without asking. Without a terminal and without either flag, it refuses. `update` does not touch `archdoc.json`, `README.md`, `AGENTS.md`, `LICENSE`, `INDEX.md`, `GLOSSARY.md` or any document.

## GitLab CI

`archdoc init` writes no GitLab configuration. Add the jobs to `.gitlab-ci.yml` yourself. This example installs a pinned release with the install script, and assumes the configured branch is `main` and `archdoc.json` is at the root of the repository:

```yaml
.archdoc:
  image: alpine:3.20
  variables:
    ARCHDOC_VERSION: v0.6.0
    GIT_DEPTH: "0"
  before_script:
    - apk add --no-cache git curl
    - curl -fsSL "https://github.com/archdochq/archdoc/releases/download/$ARCHDOC_VERSION/install.sh" | sh
    - git fetch origin "+refs/heads/main:refs/remotes/origin/main"

archdoc:
  extends: .archdoc
  script:
    - archdoc lint
    - archdoc index --check
    - archdoc glossary --check

archdoc-merge:
  extends: .archdoc
  rules:
    - if: $CI_PIPELINE_SOURCE == "merge_request_event" && $CI_MERGE_REQUEST_TARGET_BRANCH_NAME == "main"
  script:
    - archdoc lint --merge
    - archdoc index --check
    - archdoc glossary --check
    - archdoc changes
```

- `ARCHDOC_VERSION` pins the release. The install script downloads that release's archive for the runner's platform and `checksums.txt`, checks the archive against the checksums, and installs `archdoc` into `/usr/local/bin` where that is writable, and otherwise into `$HOME/.local/bin`. It needs `curl` or `wget`, `sha256sum` or `shasum`, and `tar`, and runs on Linux and macOS. Raise the pin deliberately, as with the GitHub workflow.
- `GIT_DEPTH: "0"` turns off GitLab's shallow clone, so the merge base can be found.
- The `git fetch` makes `origin/main` present whatever the pipeline fetched. `archdoc` reads it when there is no local `main`.
- `archdoc` runs in pipelines for branches, and `archdoc-merge` in merge request pipelines whose target is the configured branch. A job with `rules` runs in merge request pipelines and one without runs in branch pipelines, so each pipeline runs one of the two.
- `archdoc changes` prints its table to the job log. It is the script's last line, so it runs only when the checks before it pass, where the GitHub step runs whatever they found.

`archdoc lint --merge` always compares against the configured branch in `archdoc.json`, not the merge request's target. The condition on `CI_MERGE_REQUEST_TARGET_BRANCH_NAME` keeps it to merge requests into that branch. Write the same branch there and in the fetch.

`archdoc lint` compares frozen documents with the tip of the configured branch, wherever it runs. A branch that is behind it reports a document accepted there after the branch was cut as changed, because the branch still holds the earlier version, or as deleted where the branch does not hold it at all. That holds for branch pipelines, merge request pipelines on the source branch, the GitHub workflow's push runs and the pre-commit hook alike. Rebasing the branch, or merging the configured branch into it, clears the finding.

Where `archdoc.json` is not at the root of the repository, add `-C <directory>` to each command, such as `archdoc -C spec lint`.

## pre-commit

The repository publishes three hooks for [pre-commit](https://pre-commit.com). Add them to `.pre-commit-config.yaml`:

```yaml
repos:
  - repo: https://github.com/archdochq/archdoc
    rev: v0.6.0
    hooks:
      - id: archdoc-lint
      - id: archdoc-index
      - id: archdoc-glossary
```

| Hook | Runs |
|---|---|
| `archdoc-lint` | `archdoc lint` |
| `archdoc-index` | `archdoc index`, regenerating `INDEX.md` |
| `archdoc-glossary` | `archdoc glossary`, regenerating `GLOSSARY.md` |

`archdoc-index` and `archdoc-glossary` regenerate rather than check. When either changes its file, pre-commit stops the commit: add the regenerated file and commit again.

Each hook builds `archdoc` from source at `rev`, so the hooks are pinned as the workflow is. Keep `rev` and the workflow's `version` in step. Each hook runs from the root of the repository on every commit, whichever files the commit touches, so a commit that only deletes a frozen document is still checked.

The hooks run `archdoc lint` without `--merge`. The merge rules run in CI.

Where `archdoc.json` is not at the root of the repository, add `args: [-C, <directory>]` to each hook:

```yaml
      - id: archdoc-lint
        args: [-C, spec]
```
