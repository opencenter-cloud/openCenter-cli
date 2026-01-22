allowVolumeExpansion: true
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: {{ printf "%s-delete" ( .OpenCenter.Storage.DefaultStorageClass | default "default-delete") }}
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
parameters:
  datastoreurl: {{ .Secrets.VSphereCsi.Datastoreurl }}
provisioner: csi.vsphere.vmware.com
reclaimPolicy: Delete
volumeBindingMode: Immediate