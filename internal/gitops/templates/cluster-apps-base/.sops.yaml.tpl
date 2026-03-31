creation_rules:
{{- range .OpenCenter.GitOps.OverlayUnits.SOPS.Rules }}
  - path_regex: {{ .PathRegex | squote }}
    age: {{ join "," .AgeRecipients }}
    {{- if .EncryptedRegex }}
    encrypted_regex: {{ .EncryptedRegex | quote }}
    {{- end }}
{{- end }}
