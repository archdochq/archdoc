---
title: Glossary
includes: [ADR-0001, ADR-0002, ADR-0003]
---

# Glossary

<!-- One term per entry, alphabetical. Present tense. Terms are what things are called now; the RFC that named or renamed a term belongs in includes. -->

## backfilled

An RFC or ADR created directly in a terminal status to record a decision taken before it was written. Its front matter carries a backfilled date alongside the historical created and decided dates, and it requires a Sources section.

## editable

A document that is not frozen: any spec page, any ref, and any RFC or ADR whose status on the configured branch is not terminal.

## effectively obsolete

An RFC or ADR that an accepted document obsoletes. Obsolescence claimed by a document in any other status has no effect.

## frozen

An RFC or ADR that exists on the configured branch with a terminal status. Lint reports any change to it and every writer refuses one.

## identifier

The name a numbered document is referred to by: the uppercased type, a hyphen and a four-digit number, derived from the filename.

## implemented

An accepted RFC or ADR that some spec page other than the glossary includes.

## page

A spec page, identified by its path below spec/ without the extension.

## root

The directory under which the document directories, README.md, PROCESS.md and INDEX.md live. Named in archdoc.json relative to that file, and at or below it.

## slug

The part of a numbered document's filename after the number, derived from the title at creation and never changed by the tool.

## stale

A spec page that includes a document that is effectively obsolete.

## terminal

One of the statuses accepted, rejected or withdrawn. A document with a terminal status has no outgoing transition.
