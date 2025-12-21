allowVolumeExpansion: true
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: {{ .OpenCenter.Services.vsphere-csi.DataStore }}-delete
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
parameters:
  datastoreurl: ds:///vmfs/volumes/{{ .OpenCenter.Services.vsphere-csi.DeleteDataStoreUUID }}/
provisioner: csi.vsphere.vmware.com
reclaimPolicy: Delete
volumeBindingMode: Immediate