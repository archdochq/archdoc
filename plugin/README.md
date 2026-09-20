# ArchDoc skills

Guidance for agents documenting a project with [ArchDoc](https://github.com/archdochq/archdoc).

These skills are for working **from the code repository** and writing across into the
specification repository, which is where the evidence for backfilling lives.

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

Every file here is generated from the guides embedded in the archdoc binary, which are the same
ones `archdoc init --agents` writes into a specification repository and `archdoc agents`
refreshes. One source, three deliveries, and a test compares them byte for byte.

Do not edit these files. Change `internal/template/agents/` in the archdoc repository and run:

```
go test ./internal/template -update-plugin
```

`PROCESS.md`, in the repository being documented, is authoritative wherever anything here
disagrees with it.
