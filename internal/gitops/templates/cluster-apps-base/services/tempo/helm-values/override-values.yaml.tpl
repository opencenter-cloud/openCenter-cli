global:
  storageClass: csi-cinder-sc-delete
storage:
  trace:
    backend: s3
    s3:
      bucket: stage-cluster-tempo
      endpoint: swift.api.sjc3.rackspacecloud.com
      access_key: 51e5036b231d41f4a49e9078860e13df
      secret_key: 03710c488f864f579e5c050f0abec6f1
      region: SJC3
      insecure: false
multitenancyEnabled: true
