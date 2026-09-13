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

`spec/glossary.md` is a spec page with extra structure that lint enforces: entries are H2
headings, one paragraph each, unique, in ascending alphabetical order ignoring case. Anything
before the first H2 is preamble and is ignored.

Use the tool rather than editing the file:

```
archdoc term add "Tenant" "An isolated customer account with its own data and users."
archdoc term rename "Tenant" "Organisation"
archdoc term remove "Tenant"
archdoc term list
```

`rename` records a `Formerly *Old Name*.` line so the chain survives. It does **not** rewrite
links elsewhere, and `archdoc link` cannot repair them, because a recorded former name is not a
term. L17 will report the broken links and they are corrected by hand.

A glossary with no entries is valid. Do not invent terms to fill it.

## After any edit

```
archdoc index && archdoc lint
```

The index derives from the front matter, so any change to `includes` changes `INDEX.md`.
