---
name: archdoc-spec-pages
description: Use when editing spec pages, the glossary, or includes lists in an ArchDoc repository, covering present-tense description, keeping argument out, recording implementation, and staleness.
---

# Spec pages

The spec describes how the project works **right now**. It is always current, has no lifecycle
and no status, and is edited whenever the behaviour it describes changes.
(PROCESS.md, Document types)

Spec pages are not frozen. They are the one part of the repository you may always edit.

## Describe, never argue

> The spec describes and never argues. Argument belongs in the RFCs and ADRs a spec page
> includes.

Nothing enforces this, so it is on you.

| Write this | Not this |
| --- | --- |
| Tokens expire after 15 minutes. | Tokens expire after 15 minutes, which balances security against usability. |
| Retries use exponential backoff, capped at 5 attempts. | We chose exponential backoff because fixed intervals caused thundering herds. |
| Timestamps are stored in UTC. | Storing timestamps in UTC avoids a class of daylight-saving bugs. |

If you find yourself writing "because", "we chose", "this avoids" or "the benefit is", the
sentence belongs in the RFC or ADR that decided it. Link to that document instead.

Present tense throughout. A spec page never says "will" about behaviour that exists, and never
says anything about behaviour that does not.

## The `includes` list records implementation

A spec page's `includes` names the RFCs and ADRs the page currently reflects. This is how
implementation is recorded. (PROCESS.md, Relationships)

- An accepted RFC included by no spec page **is not yet implemented**.
- An obsoleted RFC still included by a spec page means **the page is stale**, which L12 reports.

Update `includes` in the same commit that changes the page. A page whose prose describes new
behaviour while its `includes` still names the superseded RFC is worse than either alone,
because the derived index will report something untrue.

`includes` may only name accepted documents.

## When behaviour changes

PROCESS.md, Workflow, asks that a spec page be updated in a commit referencing, by URL, the
commit or pull request in the code repository that changed the behaviour. Put that link in the
commit message. Update the `includes` list in the same commit.

## The glossary

The glossary is not a spec page. Each term is one file, `term/<slug>.md`, and `GLOSSARY.md` at
the repository root is generated from them. Never edit `GLOSSARY.md` by hand.

```yaml
---
title: effectively obsolete
formerly: []
named_by: ADR-0003
---
```

The body is the definition, exactly one paragraph, which L15 enforces. `named_by` is the
document that introduced the term. `formerly` lists previous names, oldest first.

Use the tool rather than writing the files:

```
archdoc term add "Tenant" "An isolated customer account with its own data and users." --named-by RFC-0003
archdoc term rename "Tenant" "Organisation"
archdoc term remove "Tenant"
archdoc term list
archdoc term show "Tenant"
```

`add` refuses a term whose slug matches an existing one, ignoring case. `rename` renames the
file and appends the old name to `formerly`; the generated page keeps an anchor for every former
name, so links written before the rename, including those in frozen documents, still resolve.
`remove` refuses when a frozen document links to the term.

`[[Some Term]]` resolves to `GLOSSARY.md#some-term`.

A glossary with no entries is valid. Do not invent terms to fill it.

A repository scaffolded before terms became files may still carry `spec/glossary.md`, which lint
reports as L19. Nothing is read from that page any more. Move each entry across with
`archdoc term add`, repoint links from editable documents at `GLOSSARY.md`, then delete the page.
If a frozen document links into it, leave the page in place: the links it holds can never be
repointed, and the L19 warning is then a record, not a task.

## After any edit

```
archdoc index && archdoc glossary && archdoc lint
```

The index derives from the front matter, so any change to `includes` changes `INDEX.md`. Any
change under `term/` changes `GLOSSARY.md`.
