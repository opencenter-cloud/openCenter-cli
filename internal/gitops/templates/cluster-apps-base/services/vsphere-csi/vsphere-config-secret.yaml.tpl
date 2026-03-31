apiVersion: v1
kind: Secret
metadata:
    creationTimestamp: null
    name: vsphere-csi
    namespace: vmware-system-csi
stringData:
    csi-vsphere.conf: |
        [Global]
        cluster-id = "{{ .ClusterName }}"

        [VirtualCenter "{{ .Secrets.VSphereCsi.VCenterHost }}"]
        insecure-flag = "{{ .Secrets.VSphereCsi.InsecureFlag }}"
        user = "{{ .Secrets.VSphereCsi.Username }}"
        password = "{{ .Secrets.VSphereCsi.Password }}"
        port = "{{ .Secrets.VSphereCsi.Port }}"
        datacenters = "{{ .Secrets.VSphereCsi.Datacenters }}"
