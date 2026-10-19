---
id: {{ .ID }}
title: {{ yaml .Title }}
verified: {{ .Date }}
{{- with .AsOf }}
as_of: {{ . }}
{{- end }}
---

# {{ .ID }}: {{ .Title }}

<!-- Informational only. Never normative. Cite from RFCs and ADRs by identifier. -->

## Sources

<!-- Everything the verified date is checked against. -->
