allowVolumeExpansion: true
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: {{ .OpenCenter.Services.vsphere-csi.RetainDataStoreName }}-retain
parameters:
  datastoreurl: ds:///vmfs/volumes/{{ .OpenCenter.Services.vsphere-csi.RetainDataStoreUUID }}/
provisioner: csi.vsphere.vmware.com
reclaimPolicy: Retain
volumeBindingMode: Immediate