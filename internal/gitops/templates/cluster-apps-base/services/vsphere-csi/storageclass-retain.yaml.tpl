allowVolumeExpansion: true
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: {{ printf "%s-retain" (( .OpenCenter.Storage.DefaultStorageClass | default "default") }}
parameters:
  datastoreurl: {{ .Secrets.VSphereCsi.Datastoreurl }}
provisioner: csi.vsphere.vmware.com
reclaimPolicy: Retain
volumeBindingMode: Immediate