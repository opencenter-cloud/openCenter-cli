global:

  config:
    existingSecret: "vsphere-csi"
    global:
      cluster-id: "{{ .OpenCenter.Meta.Name }}"
    csidriver:
      enabled: true
    storageclass:
      enabled: true
      name: "{{ .OpenCenter.Storage.DefaultStorageClass }}"
      storagepolicyname: ""
      expansion: true # https://vsphere-csi-driver.sigs.k8s.io/features/volume_expansion.html
      default: true
      reclaimPolicy: Delete
      volumebindingmode: "Immediate"
      datastoreurl: {{ .Secrets.VSphereCsi.Datastoreurl }}
vsphere-cpi:
  enabled: true
  global:
    config:
      existingConfig:
        enabled: true
        type: Secret
        name: "vsphere-cpi-secret"
      secretsInline: false

controller:
  config: 
    block-volume-snapshot: true
  replicaCount: 3
  snapshotter:
    image:
      registry: {{ (index .OpenCenter.Services "vsphere-csi").Image.Repository | default "registry.k8s.io" }}
      repository: sig-storage/csi-snapshotter
      tag: {{ (index .OpenCenter.Services "vsphere-csi").Image.Tag | default "v8.2.0" }}
      pullPolicy: IfNotPresent
    args:
      - "--v=4"
      - "--kube-api-qps=100"
      - "--kube-api-burst=100"
      - "--timeout=300s"
      - "--csi-address=$(ADDRESS)"
      - "--leader-election"
      - "--leader-election-lease-duration=120s"
      - "--leader-election-renew-deadline=60s"
      - "--leader-election-retry-period=30s"

snapshot:
  controller:
    enabled: true
    replicaCount: 1