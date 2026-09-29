{{/* A named template file with no chart around it. */}}
{{- define "flat.name" -}}
{{ .Values.name }}-{{ include "flat.suffix" . }}
{{- end -}}
{{- define "flat.suffix" -}}
{{ .Values.suffix | default "x" }}
{{- end -}}
{{ template "flat.name" . }}
{{- define "flat.empty" }}{{ end }}
{{ block "flat.block" . }}{{ .Values.blockDefault }}{{ end }}
{{ template ".Values.literalName" . }}
