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

## Context

<!-- The forces in play. What situation demands a decision? -->

## Decision

<!-- One paragraph, stated as a decision. "We use X" not "Choice of X". -->

## Alternatives
{{ if .Backfilled }}
Not recorded.
{{- else }}
<!-- Each alternative, and why it lost. -->
{{- end }}

## Consequences

<!-- What becomes easier, what becomes harder, what is now constrained. -->
{{- if .Backfilled }}

## Sources

<!-- What this was reconstructed from: changelog entries, commits, pull requests. Do not invent what was not recorded. -->
{{- end }}
