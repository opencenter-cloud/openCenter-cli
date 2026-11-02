---
last_updated: 2026-10-27
id: exit-codes
title: "Exit Codes"
sidebar_label: Exit Codes
description: Reference of CLI process exit codes for scripting and CI/CD, verified against cmd/exit_codes.go.
doc_type: reference
audience: "developers, devops engineers"
tags: [exit-codes, errors, automation, ci-cd]
---
# Exit Codes

**Purpose:** For automation users, documents the exit codes returned by openCenter CLI commands.

## General command contract

Most commands return an error from their `RunE`, and the root command maps it to a process exit code via `cmd/exit_codes.go`'s `ExitCode(err error) int`:

| Code | Meaning | How it's produced |
| --- | --- | --- |
| `0` | Success | `err == nil` |
| `1` | Generic error | Default fallback for any error that isn't one of the categories below (including most explicit `NewExitError(1, ...)` calls, e.g. a general OpenStack storage-apply failure) |
| `2` | Validation / usage error | `v2.IsValidationError(err)` returns true, **or** a command explicitly returns `NewExitError(2, ...)` for a usage problem (invalid/conflicting flags, a blocked plan requiring confirmation, a provider mismatch) |
| `3` | Config not found | `errors.As(err, &configNotFoundErr)` matches a `*v2.ConfigNotFoundError` (also matches when wrapped) |
| `4` | Partial completion | Currently used for exactly one case: `cluster service storage apply` returning `NewExitError(4, "OpenStack storage apply partially completed", ...)` when an apply fails after some resources were already created -- treat this as "check state before retrying," not a clean failure |

`ExitError` (`cmd/exit_codes.go`) is a small carrier type: `{Code int; Message string; Err error}`. Any command can return one to select a specific exit code; if a command's error doesn't match `*v2.ConfigNotFoundError`, isn't an `*ExitError`, and isn't a validation error, it falls through to `1`.

### Examples of code `2` usage sites

`cmd/cluster_provider_openstack.go` and `cmd/cluster_service_storage.go` both return `NewExitError(2, ...)` for: missing `--os-cloud`, invalid cluster identifier, decode/config-load failures, provider mismatch (command requires `provider: openstack` but the cluster is configured for a different provider), conflicting flag combinations (e.g. `--create-internal-network` with `--network-id`/`--subnet-id`), a blocked plan, and a structured apply invoked without `--yes`. For the provider command, a profile that lacks an application credential no longer exits 2: the credential is minted through Keystone and persisted to clouds.yaml, and a failure of that mint surfaces as a regular (non-exit-2) error.

## `opencenter secrets validate` -- a separate exit-code contract

`cmd/secrets_validate.go` does not go through `ExitCode(err)`. It calls `os.Exit(result.ExitCode)` directly with a `SyncResult.ExitCode` that is purpose-built for drift checking:

| Code | Meaning |
| --- | --- |
| `0` | No drift -- secrets match expected state |
| `1` | Drift detected |

Do not conflate this with the general contract above -- `1` here means "drift found," not "generic error."

## Practical guidance for scripts and CI

```bash
opencenter cluster validate my-cluster
case $? in
  0) echo "valid" ;;
  2) echo "validation error - check config" ;;
  3) echo "config not found" ;;
  *) echo "unexpected failure" ;;
esac
```

For `secrets validate`, only `0` and `1` are meaningful:

```bash
opencenter secrets validate my-cluster
if [ $? -ne 0 ]; then
  echo "secret drift detected"
fi
```

Exit code `4` is rare and specific to a partially-applied OpenStack storage operation -- if you see it, re-run `cluster service storage status`/inspect the target project before retrying the apply, rather than blindly retrying.
