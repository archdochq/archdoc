---
id: RFC-0007
title: Job scheduling
status: draft
created: 2026-08-14
decided:
depends: [RFC-0004]
updates: []
obsoletes: []
---

# RFC-0007: Job scheduling

## Abstract

Run jobs on a timetable, building on the queues design.

## Motivation

Work needs to happen at fixed times.

## Proposal

A scheduler, described here. A placeholder opens with `<!--`, which is prose
where it is written like that and not the start of a comment.

```yaml
# not a heading, this is inside a fence
status: example
```

## Alternatives considered

Cron, which loses on visibility.

## Backwards compatibility

Nothing breaks.

## Open questions

Whether timezones are per job.

## Changelog
