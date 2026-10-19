---
id: RFC-0015
title: Pull requests are checked against the branch they merge into
status: proposed
created: 2026-10-19
decided:
depends: [ADR-0002, ADR-0016]
updates: []
obsoletes: []
---

# RFC-0015: Pull requests are checked against the branch they merge into

## Abstract

`archdoc lint --merge` checks a pull request, as checked out merged into the configured branch, under three rules of its own: no draft lands, a document arriving with a number the branch already gives another document is named for renumbering, and every change of status is a move the lifecycle permits from the status on the branch. `archdoc changes` summarises what the pull request does to the record, and documents land on the configured branch never as drafts, with their authors proposing them before review and maintainers holding the verdicts.

## Motivation

Lint judges a tree on its own. It reads the configured branch to keep frozen documents unchanged and to set the severity of findings on them, but never asks what a pull request changes. A pull request can land a draft, a document nobody has yet offered for comment. Where it carries a number the branch has given another document since it branched, L03 reports both documents, the one already merged among them, without saying which to renumber. It can move a document between two statuses the lifecycle does not connect, as long as the end state passes lint on its own. A reviewer has no account of what the pull request changes in the record beyond the diff.

## Proposal

### Concepts

| Concept | Meaning |
|---|---|
| Base | The configured branch, resolved as L11 resolves it, with the same fallback to `origin/<branch>` |
| Merge checkout | A working tree holding the pull request merged into the base, as a pull request run checks it out |
| Arriving | A document in the working tree at a path absent from the base, unless it is numbered and the base carries its number at a path absent from the working tree. That is a rename, and the document is judged from the status of the base's document |
| Change of status | A document whose status in the working tree differs from its status at the same path on the base |
| Merge rule | A rule comparing the working tree with the base, run only under `--merge` |

### Components

| Component | Responsibility |
|---|---|
| `internal/lint` | M01, M02 and M03, as entries of `Rules` marked as merge rules, which `archdoc lint` skips without `--merge`. They read the base through the branch snapshot, which already uses the base's file listing to tell which documents are new. `Run` skips a merge rule unless the context asks for merge rules, so `archdoc lint` without `--merge` and the transition commands, which run lint before writing, are unaffected |
| `internal/changes` | The account `archdoc changes` gives, reading the base's file listing for documents removed |
| `cmd/archdoc` | `lint --merge`, and the `changes` command with `--json` |
| `archdochq/lint` | A `merge` input, which runs `archdoc lint --merge` in place of `archdoc lint`, so merge findings are annotated as every other finding is |
| `internal/template` | The scaffolded workflow, as below, for which `Data` gains the configured branch. In `PROCESS.md`: the Workflow section rewritten, the lint rules gaining the merge rules, and the commands gaining `changes` and `lint --merge`. The contributing guide: proposing before review, recording each review edit in an RFC's Changelog or an ADR's commit message, withdrawing one's own document, and its section on collisions, which gives the document as still a draft when it is renumbered. The authoring guide: a verdict following once the proposal is on the branch, pushed or in a pull request of its own. The triage guide: M01, M02 and M03. The working guide: the commands |
| Specification pages | `lint.md`: the merge rules. `commands.md`: `lint --merge`, `changes`, and the generated workflow. `lifecycle.md`: M03 judging a change by the same permitted transitions as the transition commands. `architecture.md`: `internal/changes` |
| Documentation | `README.md`: the commands, and what the generated workflow runs |
| Tests | Each merge rule against a base built in a temporary git repository, including a draft already on the base, a document proposed and accepted at once under each `strict`, a backfilled document arriving accepted, a document renamed, and a missing base; `changes` in both forms |

### Who moves a document

| Transition | Made by |
|---|---|
| `draft → proposed` | The document's author, before asking for review |
| `draft → withdrawn`, `proposed → withdrawn` | The document's author |
| `proposed → accepted`, `proposed → rejected`, and `draft → accepted` and `draft → rejected` where `strict` is off | A maintainer |

The tool does not enforce this table. The merge rules check what a pull request changes, never who made the change: git records an author as text anyone can set, so who may merge a pull request that accepts or rejects a document is decided by the code host, through branch protection and code owners.

### Merge rules

| Rule | Reports | Severity |
|---|---|---|
| M01 | An arriving RFC or ADR whose status is `draft` | Error |
| M02 | An arriving numbered document whose identifier is also carried by a document that is on the base and still in the working tree. The finding names that document's path and `archdoc renumber` | Error |
| M03 | A change of status that is not a transition PROCESS.md permits from the status on the base, under the `strict` of the base's `archdoc.json`, so a pull request cannot relax the rule it is judged by. An arriving RFC or ADR is judged as moving from `draft`, unless it carries `backfilled` | Error |

| Rule | Detail |
|---|---|
| Running | `archdoc lint --merge` runs every rule, the merge rules included, and reports them together, with the same output, `--json`, `--strict-warnings` and exit codes as `archdoc lint` |
| One step | Each change is judged as one transition from the base. Where `strict` is on, a document proposed and accepted in one pull request is refused, and so is an arriving document that is accepted or rejected without `backfilled`. The proposal reaches the branch first, and the verdict follows as a commit a maintainer pushes to the branch, or in a pull request of its own. Where `strict` is off, both are permitted, as `draft → accepted` is |
| Backfilled | An arriving document carrying `backfilled` may arrive in any terminal status, as PROCESS.md permits a backfilled document to be written already terminal |
| Drafts on the base | A draft already on the base is not arriving, and M01 does not report it. M03 does not judge an arriving draft, which M01 reports |
| Terminal on the base | M03 does not judge a document terminal on the base, whose every change L11 reports |
| Identifiers | The base's identifiers are read from its filenames, which under [ADR-0002](../adr/0002-identity-is-the-filename-number.md) carry them. A document renamed in the pull request is not arriving, so M01 and M02 do not report it, and M03 judges it from its status on the base, unless it is terminal there, where L11 reports the rename |
| Types | Only RFCs and ADRs have a status. M02 applies to every numbered type, refs included |
| No base | Where the base does not exist, or there is no git repository, `--merge` exits 1 naming the branch, as a usage error, and reports nothing |
| Merge checkout | The rules are defined on a merge checkout. On a branch not brought up to date with the base, a document the base has gained since it branched is absent from the working tree and nothing reports it. A contributor checking locally brings the configured branch up to date with the repository the pull request targets, rebases onto it, and then runs `--merge` |
| Without `--merge` | `archdoc lint` is unchanged, and never runs a merge rule |
| Rule entries | Where [RFC-0014](0014-lint-rules-and-the-command-reference-are-published-as-data.md) is built, M01, M02 and M03 carry the fields it gives every entry of `Rules` |
| Direct pushes | The merge rules apply to pull requests only. A commit pushed to the configured branch is checked by the L rules, as now, and by no merge rule, and a repository requiring every change to be reviewed does so through branch protection |

### `archdoc changes`

Prints what the working tree changes in the record against the base, as Markdown for a job summary or a pull request comment, or with `--json` as an object. Exits 0 whatever it finds; it reports, and lint judges. Defined on a merge checkout, as the merge rules are.

```markdown
## Changes to the record

| Document | Change |
|---|---|
| RFC-0015: Pull requests are checked against the branch they merge into | Arrives, proposed |
| RFC-0013: A ref can record an exchange | proposed → accepted |
| `spec/export.md` | Edited |
| `term/anchor.md` | Removed |
```

```json
{
  "base": "main",
  "changes": [
    {"type": "rfc", "id": "RFC-0013", "path": "rfc/0013-a-ref-can-record-an-exchange.md",
     "title": "A ref can record an exchange", "change": "status", "from": "proposed", "to": "accepted"}
  ]
}
```

| Change | Listed when |
|---|---|
| `arrives` | A document is at a path absent from the base, with its status in `to` where it has one |
| `status` | A document's status differs from the base, with `from` and `to` |
| `edited` | A document's file differs from the base, as git judges it, with no change of status. `terminal` is true where the document is terminal on the base |
| `removed` | A document on the base is absent from the working tree, with its status in `from` where it has one |

| Rule | Detail |
|---|---|
| Naming | A numbered document by its identifier and title, any other by its path. `id` and `title` are absent from the JSON where the document has none, and `from`, `to` and `terminal` where they do not apply |
| Order | By type, in the order RFC, ADR, spec page, ref, term, group, then by identifier, or by path for an unnumbered type |
| Renames | Not detected. A rename is a removal and an arrival, as L11 treats it |
| Generated files | `INDEX.md` and `GLOSSARY.md` are not listed |
| Nothing | The Markdown says in one line that the record is unchanged, and `changes` is an empty array |
| No base | Exits 1 naming the branch |

### The generated workflow

The lint step passes `merge` on a pull request into the configured branch, and a step after the checks writes the summary, whatever the checks found. As generated for a repository whose configured branch is `main`:

```yaml
      - uses: archdochq/lint@v1
        with:
          version: v0.7.0
          merge: ${{ github.event_name == 'pull_request' && github.base_ref == 'main' }}

      - name: Summarise the changes to the record
        if: ${{ !cancelled() && github.event_name == 'pull_request' && github.base_ref == 'main' }}
        run: archdoc changes >> "$GITHUB_STEP_SUMMARY"
```

The step carries the same `working-directory` as the other steps where the repository's `archdoc.json` is in a subdirectory. A pull request into any other branch runs lint as before. `actions/checkout` checks a pull request out merged into its base, and the workflow already fetches the full history, so the base is present. Under [ADR-0016](../adr/0016-the-scaffolded-workflow-uses-the-published-actions.md) the action installs `archdoc` and leaves it on `PATH`. `archdoc update` brings an existing workflow up to date with the template, as it does for every change to it.

### Process

PROCESS.md's Workflow section is rewritten. A document is written on a branch, offered as a pull request, and proposed by its author before review, so that it never reaches the configured branch as a draft. Edits made in review to a proposed RFC are recorded in its Changelog, and to a proposed ADR in the commit message, as for any proposed document. Discussion of a document happens on its pull request, and the sentence giving the repository's discussions as the place for it is removed. The table of who moves a document is added to the section. Its number becomes permanent when the pull request merges, and a collision before that is resolved with `renumber`. Where `strict` is on, a verdict follows once the proposal is on the branch, as a commit a maintainer pushes to it or a pull request of its own, never in the pull request that proposes the document. A backfilled document arrives already terminal, as before. The sentences giving the main branch as the place drafts are committed, and merging a draft as what assigns its number, are removed.

### Out of scope

- **Checking who made a change.** Left to the code host, as above.

## Alternatives considered

The design rests on [ADR-0016](../adr/0016-the-scaffolded-workflow-uses-the-published-actions.md), under which the merge rules run through the published action and are annotated as every other finding is, and on [ADR-0002](../adr/0002-identity-is-the-filename-number.md), under which M02 reads identifiers from the base's filenames.

- **Letting a draft merge**, as the Workflow section permits today, so that merging assigns the number. Lost because a pull request is where a document is offered for comment, so what it lands has been offered, and a draft has not.
- **Keeping proposing a maintainer's transition**, as the contributing guide gives it today. Lost because whether a document is ready for comment is its author's to say.

## Backwards compatibility

`archdoc lint` without `--merge` is unchanged, and so is every L rule. A repository whose workflow predates this design runs no merge rule until `archdoc update` brings the workflow up to date, which also needs an `archdochq/lint` release with the `merge` input. From then on, a pull request into the configured branch that lands a draft, arrives with a number taken on the base, or makes a transition the lifecycle does not permit from the base fails the check. Where `strict` is on, a document proposed and accepted in one pull request fails it, and so does a new document arriving accepted without `backfilled`; the proposal merges first, and the verdict is pushed or merged after it.

Drafts already on the configured branch are not reported, and nothing checks a direct push. The contributing guide reverses its instruction not to run `propose`, and its instruction to leave the Changelog empty.

This repository's specification is checked by its own workflow, which under [ADR-0013](../adr/0013-the-specification-is-linted-by-the-binary-built-from-the-same-commit.md) builds `archdoc` from the commit under test rather than downloading it, and gains the merge rules only if that workflow is changed to run them.

## Open questions

## Changelog
