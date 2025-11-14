---
credentials:
  extraSecretRef: "velero-credentials"
configuration:
  backupStorageLocation:
    - name: swift
      provider: community.openstack.org/openstack
      bucket: demo-cluster
  volumeSnapshotLocation:
    # for Cinder block storage
    - name: cinder
      provider: community.openstack.org/openstack-cinder
      config:
        # optional Cloud:
        #   in case clouds.yaml is used as authentication method, cloud allows
        #   user to select which cloud from the clouds.yaml to use for volume backups
        cloud: ""
        # optional Region:
        #   in case multiple regions exist in a single cloud, select which region
        #   will be used for cinder volume backups.
        region: ""
        # optional snapshot method:
        # * "snapshot" is a default cinder snapshot method
        # * "clone" is for a full volume clone instead of a snapshot allowing the
        # source volume to be deleted
        # * "backup" is for a full volume backup uploaded to a Cinder backup
        # allowing the source volume to be deleted (EXPERIMENTAL)
        # * "image" is for a full volume backup uploaded to a Glance image
        # allowing the source volume to be deleted (EXPERIMENTAL)
        # requires the "enable_force_upload" Cinder option to be enabled on the server
        method: snapshot
        # optional resource readiness timeouts in Golang time format: https://pkg.go.dev/time#ParseDuration
        # (default: 5m)
        volumeTimeout: 5m
        snapshotTimeout: 5m
        cloneTimeout: 5m
        backupTimeout: 5m
        imageTimeout: 5m
        # ensures that the Cinder volume/snapshot is removed
        # if an original snapshot volume was marked to be deleted, the volume may
        # end up in "error_deleting" status.
        # if the volume/snapshot is in "error_deleting" status, the plugin will try to reset
        # its status (usually extra admin permissions are required) and delete it again
        # within the defined "snapshotTimeout" or "cloneTimeout"
        ensureDeleted: "true"
        # a delay to wait between delete/reset actions when "ensureDeleted" is enabled
        ensureDeletedDelay: 10s
        # deletes all dependent volume resources (i.e. snapshots) before deleting
        # the clone volume (works only, when a snapshot method is set to clone)
        cascadeDelete: "true"
        # backups will be created incrementally (works only when snapshot method is set to backup)
        backupIncremental: "true"
initContainers:
  - name: velero-plugin-openstack
    image: lirt/velero-plugin-for-openstack:v0.6.0
    imagePullPolicy: IfNotPresent
    volumeMounts:
      - mountPath: /target
        name: plugins
snapshotsEnabled: true
backupsEnabled: true
