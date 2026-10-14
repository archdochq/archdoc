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

<!-- Optional. Each question a level three heading, with any detail beneath it. Once answered, it moves to a Resolved questions section after this one, ending on a blockquote holding its answer. Delete this section, or leave it empty, once nothing is open. -->

## Changelog

<!-- Optional. One entry per change made while proposed, as a list item: - YYYY-MM-DD: what changed. Continue an entry on lines indented by two spaces. -->
{{- if .Backfilled }}

## Sources

<!-- What this was reconstructed from: changelog entries, commits, pull requests. Do not invent what was not recorded. -->
{{- end }}
