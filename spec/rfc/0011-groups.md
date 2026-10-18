---
id: RFC-0011
title: Groups name the documents that belong together
status: proposed
created: 2026-10-14
decided:
depends: [ADR-0001, ADR-0003, ADR-0004, ADR-0010, ADR-0019]
updates: [RFC-0002, RFC-0004, RFC-0006, RFC-0008]
obsoletes: []
---

# RFC-0011: Groups name the documents that belong together

## Abstract

A group is a document of a new type, `group/<slug>.md`, whose front matter lists the RFCs, ADRs, refs and spec pages that belong to it and whose body describes what they have in common. Membership is held by the group rather than by its members, so a frozen document is grouped, and regrouped, without being edited. Lint checks that every member exists, `INDEX.md` lists each group with its members, and the export publishes each group and, on each document, the groups it belongs to.

## Motivation

One specification repository can describe several things at once, such as a family of applications, each built and deployed on its own. Nothing in a document says which of them it concerns. A repository that wants the division visible writes it into each title, as a prefix naming the application, where the tool cannot read it, cannot check it, and cannot correct it once the document is frozen.

An accepted document is frozen under [ADR-0001](../adr/0001-terminal-documents-are-frozen.md): a field it does not already carry, it never will. The division itself changes after the decisions inside it are taken, as one application splits into two or two merge.

The export is read by archdoc.dev and by the servers that read a record for coding agents, neither of which can tell from it which documents belong together.

## Proposal

### Concepts

| Concept | Meaning |
|---|---|
| Group | A document under `group/`, named by its slug, listing its members |
| Member | A document a group lists: an RFC, ADR or ref by its identifier, or a spec page by its path |

### Components

| Component | Responsibility |
|---|---|
| `internal/repo` | `group` as a sixth type, discovered under `group/`, with no number, no status and no required sections, and its front matter schema. Resolving a group's members, a member that is not an identifier against spec pages only, and deriving the groups each document belongs to. `Renumber` rewrites an identifier in a group's `documents` as in every other list that names documents |
| `internal/lint` | L01 checks a group's front matter. L04 checks that every member resolves and that no group lists a member twice. L10 checks a group's H1 |
| `internal/index` | A `Groups` section in `INDEX.md` where the repository has a group |
| `internal/export` | The `groups` array, `groups` on each document, and their definitions in `schema: 3` |
| `cmd/archdoc` | `export --out` writes `groups.json` where there are groups, and `index.json` carries each group without its description. `archdoc new group` is refused. The `export` command's help names `groups.json` in the tree |
| `internal/template` | In `PROCESS.md`, the document types, the front matter a group carries and the list of what lint enforces; in the spec pages guide and its description, what a group is and how it is edited; in the working guide, that a group is written by hand and which guide covers it; in the triage guide, the L04 row; in the classifying guide, that a group is not one of the types a change is classified into. The copies `archdoc update` writes follow from these |
| Published schemas | `v3.json` gains the group definition and `groups` on a document |
| Specification pages | `layout.md`: `group/`. `documents.md`: the type and its front matter. `relationships.md`: membership and the derived `groups`. `markdown.md`: a group's H1 and its lack of required sections. `lint.md`: L01, L04 and L10. `commands.md`: `new` refusing the type, and `renumber` rewriting `documents`. `index.md`: the `Groups` section. `export.md`: the shape and the tree |
| Documentation | `docs/EXPORT.md`: the shape. `docs/DECISIONS.md`: why membership is held by the group |

### A group

```markdown
---
title: Export
documents: [RFC-0004, RFC-0006, RFC-0007, RFC-0008, RFC-0009, ADR-0019, ADR-0020, export]
---

# Export

How a specification repository is published as JSON, and what may change in that shape.
```

| Key | Required | Meaning |
|---|---|---|
| `title` | Yes, not empty | The group's name as written. Its slug is the filename |
| `documents` | Yes, may be empty | The members, in any order. A member in the form of an identifier, such as `RFC-0004`, names that RFC, ADR or ref; any other member names a spec page by its path below `spec/` without the extension, and resolves only to a spec page, never to a term or a group of the same name |

The body is an H1 reading the title, as on a spec page, followed by a description of what the members have in common. A group has no required sections. Groups sit directly under `group/`, and a subdirectory there is not read, as under `term/`.

A group carries no status. It records no decision, and is edited whenever the division it records changes, whatever the status of its members. A document belongs to any number of groups, including none. A group does not list a group, and does not list a term, which is published only in the glossary under [RFC-0007](0007-the-glossary-is-a-type-of-its-own.md).

### Membership

Membership is written once, in the group. Under [ADR-0003](../adr/0003-reverse-relationships-are-derived.md) the reverse is derived: each document's groups are the groups listing it, in order of slug, as `included_in` is derived from spec pages' `includes`. `INDEX.md` publishes membership itself, in its `Groups` section. [ADR-0003](../adr/0003-reverse-relationships-are-derived.md) names the generated index as where a reverse relationship is published, but `depended_on_by` is published in the export only, and a document's derived groups are published there as it is.

| Rule | Detail |
|---|---|
| A frozen member | Grouped and regrouped by editing the group, which is never frozen |
| Renumbering | `archdoc renumber` rewrites the identifier in every group's `documents`. No group blocks a renumber, as none is frozen |
| A spec page moved or renamed, or a draft deleted | Its old name in a group no longer resolves, and L04 reports it on the group, which is edited to follow |
| Creating a group | Written by hand, as a spec page is. `archdoc new` refuses the type, as it refuses `spec` and `term`, and `archdoc init` creates no `group/` directory |

### Lint

| Rule | Applied to a group |
|---|---|
| L01 | `title` and `documents` are present, `title` is not empty, and no other key is |
| L04 | Every member resolves: an identifier to an existing RFC, ADR or ref, any other member to an existing spec page, so a member naming a term or a group is reported as not resolving. A member listed twice in one group is reported. Each finding names the member, at line 1, as L04 reports every other reference |
| L10 | The H1 reads the title, as on a spec page |

The rules about a lifecycle, numbering and required sections do not apply, as a group has none of them. The rules reading a body, such as L16 and L17, apply as to any document. A document that belongs to no group is not reported.

### Export

The envelope gains `groups`, an array of every group in order of slug, written on every export and empty where there are none. Each group carries `slug`, `title`, `path`, `description`, `documents`, its members as written, and `links`, the links in its body in the shape a document's `links` has. Each document gains `groups`, the slugs of the groups listing it, an array that is empty where there are none.

```json
"groups": [
  {
    "slug": "export",
    "title": "Export",
    "path": "group/export.md",
    "description": "\nHow a specification repository is published as JSON, and what may change in that shape.\n",
    "documents": ["RFC-0004", "RFC-0006", "RFC-0007", "RFC-0008", "RFC-0009", "ADR-0019", "ADR-0020", "export"],
    "links": []
  }
]
```

| Rule | Detail |
|---|---|
| Groups as documents | No group appears in `documents`, as no term does |
| `description` | Where the body's first heading is an H1, the body from the line after it to the end of the file, as written; otherwise the whole body. Anything before that H1 is not part of it. It is what was parsed, whatever L10 reports. Absent wherever contents bodies are: under `--no-bodies` and in `index.json`. `links` is kept in both, as a document's is |
| `--out` | `groups.json` is an array of every group, as `glossary.json` is of terms, written where there is one, with descriptions unless `--no-bodies` is given. `index.json` carries `groups` without descriptions, and an earlier export's `groups.json` is pruned where the index it left recorded groups and this export has none |
| A link to a group | A link to a group's file has `resolves_to` naming the group's slug, as a link to a term's file names the term's, and its `path` tells it from a spec page of the same name |
| Members as written | A member that does not resolve is published as written, whatever L04 reports |
| Schema | `group` is added to `$defs`, requiring `slug`, `title`, `path`, `documents` and `links`, with `description` optional. The envelope and each document require `groups` |

No tagged release has emitted `schema: 3`, so under [ADR-0019](../adr/0019-export-stability.md) the additions join the break [RFC-0008](0008-questions.md) and [RFC-0009](0009-changelog.md) already make, and `schema` stays at `3`, with `v3.json` amended. Where a release carries `schema: 3` before this design is built, the additions move it to `4`. [ADR-0019](../adr/0019-export-stability.md) counts only a removal or a change of type or meaning as a break, but every object in `v3.json` sets `additionalProperties: false`, so a consumer validating against the released `v3.json` rejects the new fields.

This amends the document object [RFC-0004](0004-exporting-the-repository-as-json.md) describes, the envelope and the `--out` tree [RFC-0006](0006-the-export-publishes-what-it-parsed.md) describes, and the `v3.json` [RFC-0008](0008-questions.md) introduces.

### The index

`INDEX.md` gains a `Groups` section after `Refs`:

```markdown
## Groups

| Group | Title | Documents |
|---|---|---|
| [export](group/export.md) | Export | [RFC-0004](rfc/0004-exporting-the-repository-as-json.md), [RFC-0006](rfc/0006-the-export-publishes-what-it-parsed.md), [RFC-0007](rfc/0007-the-glossary-is-a-type-of-its-own.md), [RFC-0008](rfc/0008-questions.md), [RFC-0009](rfc/0009-changelog.md), [ADR-0019](adr/0019-export-stability.md), [ADR-0020](adr/0020-structured-sections.md), [export](spec/export.md) |
```

| Rule | Detail |
|---|---|
| Presence | Written only where the repository has a group. The other four sections are always present |
| Rows | One per group, in order of slug. `Group` is the slug, linked to the group's file |
| `Documents` | The members in the order written, each a relative link to the document it names. A member that does not resolve is left as text, as every unresolved name in the index is, and L04 reports it |
| The description | Not shown. The group's file holds it |

### Errors

| Condition | Result |
|---|---|
| An L01 finding on a group | the group, and the key that is missing, empty or not a group's |
| An L04 finding on a group | the group, and the member that does not resolve or is listed twice |
| An L10 finding on a group | the group, the H1 and the title |
| `archdoc new group` | the refusal `spec` and `term` already get, that the type is named by its path, not numbered |

### Out of scope

- ~~**Groups in `INDEX.md`.** A group's own file lists its members and renders on GitHub.~~ Struck on 2026-10-17: GitHub renders front matter as a table of raw values, so a group's file shows its members as identifiers with no link. Brought into the design under [The index](#the-index).
- **A command that adds a document to a group.** `archdoc new rfc --group` was raised. Adding an identifier to a group's list is an ordinary edit.
- **Grouped pages and filtering in archdoc.dev and its servers.** Both read the export, and what they do with groups is their own design.

## Alternatives considered

The design rests on [ADR-0001](../adr/0001-terminal-documents-are-frozen.md), whose freeze is why membership is held by the group rather than by its members; on [ADR-0003](../adr/0003-reverse-relationships-are-derived.md), under which each document's groups are derived rather than written; on [ADR-0004](../adr/0004-the-front-matter-schema-only-gains-optional-keys.md), whose rule that every key added after `backfilled` is optional is read here as governing the types that already exist, so a new type's keys may be required. Terms already carry required `formerly` and `named_by`, added after `backfilled` when [RFC-0007](0007-the-glossary-is-a-type-of-its-own.md) made the term a type, with the reading unrecorded until this document; on [ADR-0010](../adr/0010-reference-rules-are-monotonic.md), whose rule that a valid reference stays valid is read here as protecting documents that can freeze, so L04 on a group's members, which a renamed spec page or a deleted draft breaks, need not stay satisfied, as the group is never frozen and is always edited to follow; and on [ADR-0019](../adr/0019-export-stability.md), under which the export's additions join `schema: 3`.

- **A key in each document's front matter.** A single group, or a list of tags, written on the document. Lost because a frozen document can never gain it, so every document already accepted stays ungrouped, and because a division that changes later cannot be followed in a document that cannot be edited.
- **A page of its own, generated as `GLOSSARY.md` is.** Lost because `GLOSSARY.md` holds the anchor every link to a term targets, and no link targets an anchor on a group, while a further generated file is one more for `archdoc init` to write and for a check command of its own, as `glossary --check` is for `GLOSSARY.md`.
- **A `Groups` section always present, as the other four sections are.** Lost because every repository without groups would then generate a different `INDEX.md` on upgrade, failing `index --check` until regenerated.
- **A `Groups` column in the RFC and ADR tables.** Lost because those tables already carry nine columns, and a document's groups are published in the export.

No other alternatives were weighed.

## Backwards compatibility

A repository without a `group/` directory is unchanged, apart from the export, where `groups` is empty. Its `INDEX.md` is generated as before. The required keys belong to a new type, so no existing document carries or lacks them.

A repository already holding Markdown files under `group/` has them read as groups. Each is reported by L01 where its front matter is not a group's, and by L04, L10 or L17 as any group is, and each appears in the export's `groups` and in `INDEX.md`. The directory was not read before.

The export gains `groups` on the envelope and on each document, within `schema: 3`. A consumer of `schema: 3` built before this design, validating against `v3.json` as it stood, rejects the new output.

The test comparing the schema's declared properties against the exported types checks the types in its own table, which gains the group. The test asserting that array fields are never null gains `groups` on a document, and checks of `groups` on the envelope and of a group's `documents` and `links`. `v3.json` is regenerated from the amended embedded schema. The lint fixtures gain a group with a member that does not resolve, for L04. The valid repository the tests open gains a group, so the index's golden file gains the `Groups` section and is regenerated.

## Resolved questions

### Is the type called `group`?

A repository describing a family of applications calls each one a project or a build. `group` names the mechanism rather than one use of it, and fits a repository divided by area or by theme as well.

> Yes. The type is `group`, which names the mechanism and so fits a repository divided by application, by area or by theme.

### Does lint warn about a document that belongs to no group?

Once a repository has a group, a new document written without being added to one goes unnoticed. A warning would catch it, but a repository grouping only some of its documents would carry a warning for every other one, including frozen documents, where only adding them to a group would clear it.

> No. A document in no group is not reported, so a repository groups as many or as few of its documents as it needs.

## Changelog

- 2026-10-17: Both open questions resolved: the type is called `group`, and lint does not report a document in no group.
- 2026-10-17: A group in the export carries the links in its body. Groups are listed in `INDEX.md`, struck from Out of scope.
- 2026-10-17: A member that is not an identifier resolves to a spec page only. The document updates [RFC-0002](0002-renumbering-a-document.md), [RFC-0006](0006-the-export-publishes-what-it-parsed.md) and [RFC-0008](0008-questions.md), and depends on [ADR-0004](../adr/0004-the-front-matter-schema-only-gains-optional-keys.md). `groups.json` follows `--no-bodies`, a group's description is defined where its body has no H1, and the tests named in Backwards compatibility are corrected.
- 2026-10-17: A link to a group's file resolves to its slug. A document's derived groups are published in the export only, as `depended_on_by` is.
- 2026-10-18: Alternatives that were not weighed are removed: directories, the prefix of a title, spec pages' `includes`, and one file listing every group.
- 2026-10-18: The document depends on [ADR-0010](../adr/0010-reference-rules-are-monotonic.md), read as protecting documents that can freeze, and records its reading of [ADR-0004](../adr/0004-the-front-matter-schema-only-gains-optional-keys.md) as its own. A group's description starts after the first heading where that is an H1, `groups.json` is an array, and the components gain `markdown.md`, the working guide and the export command's help.
