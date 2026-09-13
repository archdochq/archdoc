---
id: RFC-0010
title: Queue durability
status: proposed
created: 2026-06-02
decided:
depends: []
updates: []
obsoletes: []
---

# RFC-0010: Queue durability

## Abstract

Decide how queued work survives a restart.

## Motivation

Work in flight is lost today.

## Proposal

Persist the queue.

## Alternatives considered

Accepting the loss, which is cheaper but wrong.

## Backwards compatibility

Nothing breaks.

## Open questions

Whether the persistence is synchronous.

## Changelog

- 2026-06-02: raised.
