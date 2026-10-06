{{- define "crswd.reconcilerNamespace" -}}
{{- default (printf "%s-reconciler" .Release.Namespace) .Values.reconciler.namespace -}}
{{- end -}}

{{- define "crswd.image" -}}
{{- printf "%s:%s" .Values.image.repository (default .Chart.AppVersion .Values.image.tag) -}}
{{- end -}}

{{- define "crswd.sessionImage" -}}
{{- printf "%s:%s" .Values.sessionImage.repository (default .Chart.AppVersion .Values.sessionImage.tag) -}}
{{- end -}}

{{/* Called as (dict "root" . "component" "daemon"). */}}
{{- define "crswd.selectorLabels" -}}
app.kubernetes.io/name: crswd
app.kubernetes.io/instance: {{ .root.Release.Name }}
app.kubernetes.io/component: {{ .component }}
{{- end -}}

{{- define "crswd.labels" -}}
{{ include "crswd.selectorLabels" . }}
app.kubernetes.io/part-of: crswd
helm.sh/chart: {{ printf "%s-%s" .root.Chart.Name .root.Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end -}}

{{/* Refuses the installs FR-009, FR-010 and FR-013 forbid. Included once, from daemon.yaml. */}}
{{- define "crswd.validate" -}}
{{- if eq (include "crswd.reconcilerNamespace" .) .Release.Namespace -}}
{{- fail (printf "reconciler.namespace %q equals the release namespace: the daemon holds pods/exec there and the reconciler creates the session pods there. Use a different namespace (the default is <release namespace>-reconciler)" .Release.Namespace) -}}
{{- end -}}
{{- $_ := required "sessionNode is required: the node that holds the ReadWriteOnce claim, so every session pod runs on it (--set sessionNode=<node>)" .Values.sessionNode -}}
{{- end -}}
