---
title: Lifecycle
includes: [ADR-0001]
---

# Lifecycle

RFCs and ADRs have a status. Spec pages and refs do not.

## Statuses

`draft`, `proposed`, `accepted`, `rejected` and `withdrawn`. The last three are terminal. A document with a terminal status is frozen: any change to it once it exists on the configured branch is reported by lint, and every writer refuses it.

## Transitions

In strict mode, the default:

| From | To |
|---|---|
| `draft` | `proposed`, `withdrawn` |
| `proposed` | `accepted`, `rejected`, `withdrawn` |
| `accepted` | none |
| `rejected` | none |
| `withdrawn` | none |

Non-strict mode adds `draft → accepted` and `draft → rejected`. No other transition exists in either mode. `proposed → draft` is never permitted.

## Transition commands

`archdoc propose <id>`, `accept <id>`, `reject <id>` and `withdraw <id>` each:

1. Load the document. Fail if the identifier is unknown or the type is not RFC or ADR.
2. Check the transition is permitted from the current status under the `strict` setting. Fail naming the transitions permitted from the current status.
3. For `accept` and `reject`: fail if there is no H1, or it does not read as the front matter gives it; fail if a required section is missing, or appears before one it must follow; fail if any required section is empty, other than `Open questions` and `Changelog`; fail if any unresolved `[[...]]` remains. For `accept`: fail if the document has an `Open questions` section and it is non-empty. `withdraw` performs no content checks.
4. Rewrite `status` and, for a terminal transition, `decided` to today. Front matter is rewritten field by field, preserving order and comments. The body is not touched except in step 5.
5. For `reject`: append `## Rejection rationale` to the end of the body with an HTML comment prompting for the rationale. The step 3 checks run before this append.
6. Print the new status.

A transition refuses to write a `decided` date earlier than `created`, and refuses a status the graph does not know. `reject` refuses a document whose parse stops short of the end of the file. None of the transition commands commits.

## Backfilling

`archdoc new --backfill` creates a document already in a terminal status, with `created` and `decided` set to the dates given and `backfilled` set to today. A backfilled document uses no transition. It requires a `Sources` section, and `Alternatives considered` and `Backwards compatibility` are pre-filled with `Not recorded.`.
