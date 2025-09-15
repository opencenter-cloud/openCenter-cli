# GitOps Templates

This directory contains starter files used to bootstrap a GitOps repository. All files are embedded into the openCenter binary via Go’s `embed` directive. During `cluster setup` or `cluster render` the files are copied or rendered into the directory specified by `gitops.git_dir`.

## Rendering Rules

- Files ending with `.tmpl` are treated as Go `text/template` files. They are executed with the entire cluster configuration bound to the template context (see `internal/config/config.go` for field names). The [Sprig](https://masterminds.github.io/sprig/) function library is available for utility functions (string manipulation, arithmetic, etc.). The output file name is the same as the source but without the `.tmpl` extension.
- Files without the `.tmpl` suffix are copied verbatim.
- Subdirectories are preserved.

You can add your own templates here. For example, to reference the cluster name and git directory in a YAML file:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: {{ .ClusterName }}
  annotations:
    gitops-dir: {{ .GitOps.GitDir }}
```