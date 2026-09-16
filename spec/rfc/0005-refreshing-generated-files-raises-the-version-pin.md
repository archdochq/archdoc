---
id: RFC-0005
title: Refreshing generated files raises the version pin
status: accepted
created: 2026-09-16
decided: 2026-09-16
backfilled: 2026-09-16
depends: []
updates: [RFC-0003]
obsoletes: []
---

# RFC-0005: Refreshing generated files raises the version pin

## Abstract

`archdoc update` raises the workflow's `ARCHDOC_VERSION` to the version doing the update, holding the existing pin only when the running binary is not a published release. This amends the pin rule of [RFC-0003](0003-refreshing-generated-files.md).

## Motivation

The command rewrites `PROCESS.md`, which describes what lint enforces. Refreshing that description while leaving CI pinned to an older ArchDoc leaves a repository documenting one set of rules and enforcing another. A repository four releases behind was found with exactly that arrangement, its owner having raised the pin by hand to the value the command declined to write.

## Proposal

### The rule

| Running binary | Written as `ARCHDOC_VERSION` |
|---|---|
| A published release | the running binary's version |
| Not a published release, and a pin exists | the existing pin |
| Not a published release, and no pin exists | `latest` |

A version is a published release when it is a release tag and not a pseudo-version. The change appears in the diff like any other and is confirmed the same way.

### Steps

Step 2 of [RFC-0003](0003-refreshing-generated-files.md) renders the workflow with the version chosen by the table above. Every other step is unchanged.

### Out of scope

- Any other file. Only the workflow carries a pin.

## Alternatives considered

- **Keeping the pin as it was.** The rule this replaces. Its reason held only for an unreleased binary, which would write `latest` over a real version, and the table keeps that case.

No other alternatives were weighed.

## Backwards compatibility

The behaviour of `archdoc update` changes. A repository pinned to one release and updated with a later one previously kept its pin; it now takes the later release. The diff shows the change before it is written.

## Open questions

## Changelog

## Sources

- [da4fd10] makes the change and sets `created` and `decided`. Its message states the documenting-one-thing-enforcing-another argument.
- `docs/ARCHDOC.md` at [da4fd10a], section `archdoc update`, describes the amended rule.
- The repository found four releases behind, and its owner's hand edit matching the command's later output, are taken from what followed rather than anticipated.

[da4fd10]: https://github.com/ollieread/archdoc/commit/da4fd10c460c036ed8a7827ed74b0925138d73ca
[da4fd10a]: https://github.com/ollieread/archdoc/blob/da4fd10c460c036ed8a7827ed74b0925138d73ca/docs/ARCHDOC.md
