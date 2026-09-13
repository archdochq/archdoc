---
id: {{ .ID }}
title: {{ yaml .Title }}
status: {{ .Status }}
created: {{ .Date }}
decided:{{ with .Decided }} {{ . }}{{ end }}
{{- with .Backfilled }}
backfilled: {{ . }}
{{- end }}
depends: []
updates: []
obsoletes: []
---

# {{ .ID }}: {{ .Title }}

## Abstract

<!-- Two or three sentences. What does this propose? -->

## Motivation

<!-- What is wrong or missing today, and why does it matter now? Link the GitHub Discussion here once proposed. -->

## Proposal

<!-- The design. This is the bulk of the document. -->

## Alternatives considered
{{ if .Backfilled }}
Not recorded.
{{- else }}
<!-- What else was on the table and why it lost. Significant alternatives that are decisions in their own right go in an ADR; link it here. -->
{{- end }}

## Backwards compatibility
{{ if .Backfilled }}
Not recorded.
{{- else }}
<!-- What breaks. "Nothing" is an acceptable answer. -->
{{- end }}

## Open questions

<!-- Must be empty before acceptance. Anything remaining moves to a follow-up RFC. -->

## Changelog

<!-- One dated line per change made while proposed. -->
{{- if .Backfilled }}

## Sources

<!-- What this was reconstructed from: changelog entries, commits, pull requests. Do not invent what was not recorded. -->
{{- end }}
