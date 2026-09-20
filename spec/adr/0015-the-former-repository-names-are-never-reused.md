---
id: ADR-0015
title: The former repository names are never reused
status: accepted
created: 2026-09-20
decided: 2026-09-20
depends: [ADR-0001]
updates: []
obsoletes: []
---

# ADR-0015: The former repository names are never reused

## Context

Raised on 2026-09-20, when the project moved from a personal account to the `archdochq` organisation. `ollieread/archdoc` became `archdochq/archdoc`, and `ollieread/homebrew-tap` became `archdochq/homebrew-tap`.

Eighteen of the twenty terminal documents in this repository cite commits and blobs under `github.com/ollieread/archdoc` in their Sources. Under [ADR-0001](0001-terminal-documents-are-frozen.md) none of them can be edited, so those citations cannot be rewritten to the new path. They resolve because GitHub redirects the URLs of a transferred repository, and that redirect holds only while no repository of the old name exists. Creating one replaces it, and every citation in the record stops resolving at the same moment.

The tap has the same shape with a smaller consequence. Anyone who has run `brew tap ollieread/tap` holds a clone that keeps fetching through the same redirect, and recreating the name leaves them on whichever release was current when the transfer happened, without telling them.

## Decision

Neither `ollieread/archdoc` nor `ollieread/homebrew-tap` is created again, by this project or for anything else. The names stay unused for as long as the record is expected to be readable.

## Alternatives

- **Rewriting the citations to the new path.** [ADR-0001](0001-terminal-documents-are-frozen.md) forbids it. A terminal document is never modified, and eighteen of them carry these links.
- **Recreating `ollieread/archdoc` as a signpost, holding a README naming where the project went.** A repository at the name replaces the redirect, so every link into a commit or a blob stops resolving even though the name now holds something helpful. It answers the reader who types the old name and breaks the reader following a citation, which is the one the record exists for.
- **Accepting the dependency and reserving nothing.** The same outcome as the signpost, reached by accident rather than on purpose, and at a time nobody chooses.

## Consequences

Easier:

- Every citation in the record keeps resolving with no action taken and no document edited.

Harder:

- Nothing may ever be published under either name again, including work unrelated to this project.
- The record's citations rest on a third party's redirect, which this project does not control and cannot check. Nothing here detects the day one stops working.

Constrained:

- Any later move of either repository will add the name it leaves behind to this list, on the same reasoning.
- Documents written from here on will cite whichever path is in force when they are written, so the record will accumulate citations under more than one owner, each depending on its own redirect.
