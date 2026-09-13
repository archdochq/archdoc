---
id: ADR-0001
title: Storage engine
status: accepted
created: 2026-01-20
decided: 2026-02-05
depends: []
updates: []
obsoletes: []
---

# ADR-0001: Storage engine

## Context

Several engines were available and the choice constrains every later design.

## Decision

We use PostgreSQL.

## Alternatives

MySQL, which lost on extension support.

## Consequences

Every spec page may assume PostgreSQL semantics.
