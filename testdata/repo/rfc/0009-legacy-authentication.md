---
id: RFC-0009
title: Legacy authentication
status: accepted
created: 2023-04-02
decided: 2023-05-10
backfilled: 2026-09-11
depends: []
updates: []
obsoletes: []
---

# RFC-0009: Legacy authentication

## Abstract

Sessions are held server-side and identified by an opaque cookie. This records
a decision taken in 2023 and reconstructed from the changelog.

## Motivation

The service needed authentication before any of this repository existed.

## Proposal

Server-side sessions, an opaque cookie, no token in client storage.

## Alternatives considered

Not recorded.

## Backwards compatibility

Not recorded.

## Open questions

## Changelog

## Sources

- CHANGELOG.md entry for 0.4.0, dated 2023-05-10.
- The commit that added the session store.
