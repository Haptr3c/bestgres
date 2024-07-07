{{- define "envVars" -}}
- name: WATCH_NAMESPACE
  valueFrom:
    fieldRef:
      fieldPath: metadata.namespace
- name: OPERATOR_IMAGE
  value: {{ include "operatorImage" . }}
- name: MODE
  value: operator
{{/*
TODO: Remove below this line if not needed
*/}}
- name: OPERATOR_NAME
  value: "{{ .Release.Name }}"
- name: POD_NAME
  valueFrom:
    fieldRef:
      fieldPath: metadata.name
- name: POD_NAMESPACE
  valueFrom:
    fieldRef:
      fieldPath: metadata.namespace
{{- end -}}