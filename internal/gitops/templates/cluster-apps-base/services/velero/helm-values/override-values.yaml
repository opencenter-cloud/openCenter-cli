---
credentials:
  extraSecretRef: "cloud-credentials"

configuration:
  features: EnableCSI
  defaultSnapshotMoveData: false
  defaultVolumesToFsBackup: false
  backupStorageLocation:
    - name: {{ .OpenCenter.OpenStack.Region }}
      provider: community.openstack.org/openstack
      default: true
      bucket: {{ .OpenCenter.Cluster.Name }}-velero
      config:
        region: {{ .OpenCenter.OpenStack.Region }}
  volumeSnapshotLocation: []
initContainers:
  - name: velero-plugin-openstack
    image: lirt/velero-plugin-for-openstack:v0.6.0
    imagePullPolicy: IfNotPresent
    volumeMounts:
      - mountPath: /target
        name: plugins
snapshotsEnabled: true
backupsEnabled: true
deployNodeAgent: false

extraObjects:
  - apiVersion: snapshot.storage.k8s.io/v1
    kind: VolumeSnapshotClass
    metadata:
      name: velero-vsphere-snapshot-class
      labels:
        velero.io/csi-volumesnapshot-class: "true"
    driver: csi.vsphere.vmware.com
    deletionPolicy: Delete