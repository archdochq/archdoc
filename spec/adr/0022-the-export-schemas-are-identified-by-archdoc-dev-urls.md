---
id: ADR-0022
title: The export schemas are identified by archdoc.dev URLs
status: accepted
created: 2026-10-18
decided: 2026-10-18
depends: [ADR-0019]
updates: []
obsoletes: []
---

# ADR-0022: The export schemas are identified by archdoc.dev URLs

## Context

Raised on 2026-10-15, before any tagged release had emitted `schema: 3`.

Under [RFC-0004](../rfc/0004-exporting-the-repository-as-json.md) a schema's `$id` is the URL of its published copy, the file in the source repository. The v3 schema gives its `$id` as `https://raw.githubusercontent.com/archdochq/archdoc/main/schema/export/v3.json`, which names the code host, the repository's owner and the `main` branch.

The first release to emit `schema: 3` embeds its `$id`, and under [ADR-0019](0019-export-stability.md) that schema stays published, so the identifier it carries has to keep reaching it. A schema can also gain optional fields without its number changing, so a later release can publish a `v3.json` that differs from the first. The identifiers carried by earlier releases have moved with the repository: the pre-releases from `v0.1.0-alpha.4` to `v0.1.0-alpha.6` named a path that was not a file, `v0.1.0` named the file under the repository's first owner, and `v0.2.0` onwards name it under `archdochq`. The URLs under the first owner still resolve through GitHub's redirect of a transferred repository, which under [ADR-0015](0015-the-former-repository-names-are-never-reused.md) holds only while no repository of the old name exists.

`archdoc.dev` is the project's site, and under [ADR-0018](0018-the-module-path-is-the-project-s-own-domain.md) it already answers for the module path, so that a change of code host changes what the site serves rather than the path.

## Decision

From `schema: 3`, an export schema's `$id` is `https://archdoc.dev/schema/export/v<n>.json`, where `<n>` is its `schema` number. The source repository still publishes each schema at `schema/export/v<n>.json`, and the site serves each schema from v3 onwards at its `$id`. `v1.json` and `v2.json` are released, and keep their `$id` and their path in the source repository.

## Alternatives

- **Keeping the URL on `main`.** Lost because it names the code host, and a change of host would leave every released v3 schema identified by a URL that no longer reaches it. A change of owner, which has happened once, would leave it reachable only through GitHub's redirect.
- **A URL at the tag of the first release to emit `schema: 3`.** The file it fetches never changes. Lost because it still names the code host and the owner, as the URL on `main` does, and because a later release that adds optional fields to v3 would carry an identifier fetching the copy without them.

## Consequences

Easier:

- A change of code host or owner will change where the site's copy comes from, not the identifier of any schema from v3 onwards.

Harder:

- Until the site serves `/schema/export/v3.json`, the v3 `$id` will reach nothing, and a tool resolving it will get an error page.
- The schemas will be identified on two hosts: v1 and v2 on GitHub, v3 onwards on `archdoc.dev`.

Constrained:

- The site will serve each schema from v3 onwards at its `$id`, for as long as the schema stays published. Losing the domain will leave every schema from v3 onwards identified by a URL that reaches nothing.
