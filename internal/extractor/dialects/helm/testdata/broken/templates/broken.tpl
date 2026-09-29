{{- define "broken.before" -}}
{{ .Values.before.error }}
{{- end }}

{{- define "broken.open" -}}
{{ if .Values.unterminated }}
{{ include "broken.before" . }}

{{- define "broken.after" -}}
{{ .Values.after }}
{{- end }}
