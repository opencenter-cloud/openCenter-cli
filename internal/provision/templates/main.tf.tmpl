{{- /*
  Terraform main.tf template rendered from Config.IAC.Main (locals) and Config.IAC.Modules (module blocks).
  Keys are rendered in stable order. Values are formatted to HCL via the `hcl` helper.
*/ -}}

{{- $locals := .IAC.Main -}}
locals {
{{- range $k := $locals | keys | sortAlpha }}
  {{ $k }} = {{ hcl (index $locals $k) }}
{{- end }}
}

{{- $mods := .IAC.Modules -}}
{{- range $name := $mods | keys | sortAlpha }}
module "{{ $name }}" {
  {{- $attrs := (index $mods $name) -}}
  {{- range $k := $attrs | keys | sortAlpha }}
  {{ $k }} = {{ hcl (index $attrs $k) }}
  {{- end }}
}
{{- end }}
