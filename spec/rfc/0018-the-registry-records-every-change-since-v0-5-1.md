---
id: RFC-0018
title: The registry records every change since v0.5.1
status: proposed
created: 2026-11-02
decided:
depends: []
updates: [RFC-0014]
obsoletes: []
---

# RFC-0018: The registry records every change since v0.5.1

## Abstract

Each rule's `Changes` records every change to what the rule reports in a release after v0.5.1, the last release tagged before the registry was built, including the changes made before the registry existed. Changes in v0.5.1 and earlier releases are not recorded. This replaces the History rule of [RFC-0014](0014-lint-rules-and-the-command-reference-are-published-as-data.md).

## Motivation

[RFC-0014](0014-lint-rules-and-the-command-reference-are-published-as-data.md) gives `Changes` starting empty for every rule and recording the changes made once its design is built, with the changes made before it not recorded. The registry was built on 2026-10-26, after v0.5.1 was tagged on 2026-09-23. The rule changes made between those dates are in no tagged release, and v0.6.0 is the first to carry them: findings reported at the line of their field rather than line 1, groups, a ref's `as_of`, and `Open questions` and `Changelog` no longer required in an RFC, among others. Under the History rule, v0.6.0's entries list only what changed after 2026-10-26, and each rule's page publishes an account of v0.6.0 that leaves out most of what v0.6.0 changes in it.

[RFC-0014](0014-lint-rules-and-the-command-reference-are-published-as-data.md) already gives L20 and L21 a `Since` of v0.6.0, the release carrying them, though both were built before the registry.

## Proposal

| Rule | Detail |
|---|---|
| Recorded releases | `Changes` records every change to what a rule reports in a release after v0.5.1, under the first release carrying it |
| Earlier releases | A change in v0.5.1 or an earlier release is not recorded |
| Changes made before the registry | A change made after v0.5.1 and before the registry was built is recorded under v0.6.0. Its entry is written from the code at v0.5.1 and the changes made since, once |
| Changes made since | A change made once the registry is built is recorded with the change, as [RFC-0014](0014-lint-rules-and-the-command-reference-are-published-as-data.md) gives it |

This replaces the History row of [RFC-0014](0014-lint-rules-and-the-command-reference-are-published-as-data.md)'s rule entry table. The rest of that design stands.

A test in `internal/lint` names each of v0.6.0's entries by a phrase of its text and checks each rule's number of v0.6.0 entries, so an entry deleted, or moved to another release, is reported. It does not report a change that was never recorded: nothing checks that a change to what a rule reports is recorded in `Changes`, as in [RFC-0014](0014-lint-rules-and-the-command-reference-are-published-as-data.md).

The lint spec page's account of `Changes` says that changes in v0.5.1 and earlier releases are not listed.

## Alternatives considered

- **Recording only the changes made once the registry is built**, as [RFC-0014](0014-lint-rules-and-the-command-reference-are-published-as-data.md) gives it. Lost because v0.6.0's published changes would leave out most of what v0.6.0 changes.
- **Recording the changes made in releases already tagged**, which [RFC-0014](0014-lint-rules-and-the-command-reference-are-published-as-data.md) rejects because each entry would be reconstructed from the code at each tag, and a wrong one published on the rule's page. Still rejected. Recording from v0.5.1 reconstructs the changes of one release not yet tagged, against one tag.

No other alternatives were weighed.

## Backwards compatibility

Nothing breaks. No tagged release has published the registry, so no published `changes` entry is altered.
