---
id: localdev-rustfs
title: "Local RustFS development service"
sidebar_label: Local RustFS
description: Reference for the local RustFS development service and its credential and Kind attachment workflow.
doc_type: reference
audience: "contributors, maintainers"
tags: [local-development, rustfs, reference]
last_updated: 2026-09-25
---
# Local RustFS development service

**Purpose:** For contributors, maintainers, documents the local RustFS development service and its credential and Kind attachment workflow.

`opencenter-local rustfs up` starts the pinned RustFS image with its S3 API on
`127.0.0.1:9000` and console on `127.0.0.1:9001`. State, object data, and the
private generated credentials file live below the local state directory.

The default image is the immutable reference
`docker.io/rustfs/rustfs:1.0.0@sha256:8cc9801755448b71a786705ce76692c77e14936cccd87cf2fc31842e58f4d1ff`.

Credentials are generated once and persisted. A configured credential file may
be supplied with `--credentials-file path.json`, or safely through stdin with
`--credentials-file -`:

```json
{"access_key":"example-access","secret_key":"example-secret"}
```

`status` never prints credential values. RustFS containers are identified by
openCenter ownership labels and non-secret specification/credential hashes;
containers with missing ownership or drift are not adopted or removed.

`up` does not auto-attach to Kind. Use `attach-kind --cluster` so the
authenticated temporary-pod probe completes before attachment is reported
successful.

`attach-kind` validates the authenticated S3 endpoint from a disposable
temporary AWS CLI pod scheduled through the supplied Kind kubeconfig. The
service also reports a host-side health/S3 check through the container's
Kind-network IP. Failure of the pod probe fails attachment; a live Docker,
Podman, and Kind environment is required for this integration check.

The probe uses the immutable image
`public.ecr.aws/aws-cli/aws-cli:2.36.22@sha256:5a4cc81c75d7b08ba6daa9439599812d00ed3b497a82a9b4bdc771a77c74268c`.
