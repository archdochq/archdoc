---
id: ADR-0023
title: Every finding no permitted edit can clear is a warning
status: proposed
created: 2026-10-31
decided:
depends: [ADR-0001, ADR-0009]
updates: [ADR-0009]
obsoletes: []
---

# ADR-0023: Every finding no permitted edit can clear is a warning

## Context

[ADR-0009](0009-a-lint-error-names-something-the-user-may-change.md) makes a finding that no permitted edit to the named document could clear a warning, and applies it to the rules whose subject is the document's own body, naming L14, L16 and L17. The rules added since that read a body, L20 and L21, follow it. Six others report faults inside a frozen document that only an edit to that document could clear, and under [ADR-0001](0001-terminal-documents-are-frozen.md) that edit is never permitted:

| Rule | A fault it reports in a frozen document |
|---|---|
| L01 | A required front matter key missing, or a value of the wrong type |
| L02 | An `id` differing from the identifier the filename gives |
| L07 | A `decided` date before `created` |
| L08 | A required section missing or out of order |
| L09 | A question left under `Open questions` in an accepted document |
| L10 | An H1 differing from the title |

Each stays an error, so lint fails on that repository for good. L08, L09 and L10 read the body, and ADR-0009's list omits them. L01, L02 and L07 read the front matter, which ADR-0009's scope does not reach. The gap was raised by an audit of the tool on 2026-10-26, recorded as [#30].

## Decision

A finding that no permitted edit to the named document could clear is a warning, whatever part of the document it concerns. When the document is frozen this applies to L01, L02, L07, L08, L09, L10, L14, L16, L17, L20 and L21. It does not apply to a fault in a relationship between documents, which is cleared by editing the other document, nor to L11, whose finding is that a frozen document was edited and whose remedy is to revert.

## Alternatives

- **The body rules alone.** L08, L09 and L10 become warnings and L01, L02 and L07 stay errors, as ADR-0009's scope gives it. A front matter fault in a frozen document is as permanent as a body fault, so this keeps the failure ADR-0009 was written to remove.

No other alternatives were weighed.

## Consequences

Easier:

- A repository holding a frozen document with any fault in it will still pass lint, and the fault will still be reported.
- Whether a finding is an error will follow from one question about the named document, not from a list of rules.

Harder:

- More of what lint prints will be warnings that can never be cleared.

Constrained:

- A rule added later will report a warning wherever its finding in a frozen document could only be cleared by editing that document.

[#30]: https://github.com/archdochq/archdoc/issues/30
