---
id: ADR-0002
title: No ORM
status: rejected
created: 2026-03-02
decided: 2026-03-18
depends: [ADR-0001]
updates: []
obsoletes: []
---

# ADR-0002: No ORM

## Context

Hand-written SQL was proposed throughout.

## Decision

We would write SQL by hand everywhere.

## Alternatives

An ORM, which was said to obscure queries.

## Consequences

Every query would be written twice.

## Rejection rationale

The cost fell on every contributor and the benefit was confined to three queries.
