---
id: ADR-0012
title: The licence is AGPL-3.0-or-later
status: accepted
created: 2026-09-13
decided: 2026-09-13
backfilled: 2026-09-16
depends: []
updates: []
obsoletes: []
---

# ADR-0012: The licence is AGPL-3.0-or-later

## Context

The tool is to be open source, and nobody is to be able to freely make money from it. Those two requirements conflict as stated: the Open Source Definition forbids discriminating against commercial use by name, so a licence that prevents selling is not an open source licence. The tool is meant to live inside company repositories and scaffold their continuous integration, so a licence that a company's policy refuses defeats its purpose. It is a program that is run, not a library that is linked, so a copyleft obligation lands on someone redistributing a modified tool and on nobody merely using it.

## Decision

The licence is the GNU Affero General Public License, version 3 or later. Anyone may use the tool commercially. Anyone who redistributes a modified version, or runs one as a network service, must publish their source under the same terms. The copyright is held by one party, so that a commercial exemption could be sold alongside the copyleft. The `LICENSE` that `archdoc init --license=mit` writes into a scaffolded repository covers that repository's documents and has no bearing on this.

## Alternatives

- **Plain GPL-3.0.** It leaves open the one route that matters, a hosted specification-management service built on a private fork, because its obligations attach to distribution and a service distributes nothing. Section 13 of the AGPL closes that route, and binds only someone offering a modified version over a network, which a locally run command never does.
- **A source-available licence: BUSL, FSL or PolyForm.** Each forbids commercial use outright, which is closer to the literal requirement, but none is an open source licence and each is one that corporate policy commonly blocks.

## Consequences

Easier:

- The tool is open source under the definition, so no policy that admits open source software refuses it.
- Nobody can take a modified tool closed and sell it, or offer one as a service without publishing the source.

Harder:

- Commercial use is permitted, not prevented. Someone charging to set the tool up, or using it inside a business, owes nothing.

Constrained:

- Accepting a contribution without a licence agreement ends the possibility of selling a commercial exemption, because the copyright would no longer be held by one party.
- The licence text ships in every release archive, as the AGPL requires of a distributor.

## Sources

- `docs/DECISIONS.md` at [9654fe2d], section Licence, records the requirement, the conflict with the Open Source Definition, the choice of Affero over plain GPL, the rejection of source-available licences, and the single copyright holder. This sets `created` and `decided`.
- `LICENSE` at [9654fe2l] is the licence text as adopted.
- The decision was taken in a design session on 2026-09-13, not publicly available, and first written down the same day.

[9654fe2d]: https://github.com/ollieread/archdoc/blob/9654fe2b9cdc3041294e99a64ebe6e61477e8bcc/docs/DECISIONS.md
[9654fe2l]: https://github.com/ollieread/archdoc/blob/9654fe2b9cdc3041294e99a64ebe6e61477e8bcc/LICENSE
