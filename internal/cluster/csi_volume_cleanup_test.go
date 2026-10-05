// Copyright 2025 Dinesh Gupta <dinesh.gupta@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cluster

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

// fakeCleanupRunner records calls and returns configurable output.
type fakeCleanupRunner struct {
	kubectlOut []byte
	kubectlErr error
	calls      []commandCall
}

func (r *fakeCleanupRunner) Run(_ context.Context, dir string, env map[string]string, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, commandCall{dir: dir, env: env, name: name, args: args})
	if name == "kubectl" {
		return r.kubectlOut, r.kubectlErr
	}
	return nil, nil
}

// fakeCinderSvc is the cinder service double for cleanup tests.
type fakeCinderSvc struct {
	// volumes maps volume ID → status and size.
	volumes map[string]struct {
		status string
		size   int
	}
	deleted  []string
	deleteErr map[string]error
}

func (f *fakeCinderSvc) GetStatus(_ context.Context, id string) (string, int, error) {
	if v, ok := f.volumes[id]; ok {
		return v.status, v.size, nil
	}
	return "", 0, fmt.Errorf("volume %s not found", id)
}

func (f *fakeCinderSvc) Delete(_ context.Context, id string) error {
	if err, ok := f.deleteErr[id]; ok {
		return err
	}
	f.deleted = append(f.deleted, id)
	return nil
}

// ---------------------------------------------------------------------------
// collectCSIVolumeHandles tests
// ---------------------------------------------------------------------------

func TestCollectCSIVolumeHandles_Success(t *testing.T) {
	dir := t.TempDir()
	kubeconfigPath := filepath.Join(dir, "kubeconfig.yaml")
	if err := os.WriteFile(kubeconfigPath, []byte("placeholder"), 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}

	pvJSON := buildPVJSON([]string{"vol-aaa", "vol-bbb"})
	runner := &fakeCleanupRunner{kubectlOut: pvJSON}

	handles, err := collectCSIVolumeHandles(context.Background(), kubeconfigPath, runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(handles) != 2 {
		t.Fatalf("expected 2 handles, got %d: %v", len(handles), handles)
	}

	seen := map[string]bool{}
	for _, h := range handles {
		seen[h] = true
	}
	for _, want := range []string{"vol-aaa", "vol-bbb"} {
		if !seen[want] {
			t.Errorf("expected handle %q in result %v", want, handles)
		}
	}

	// Verify kubectl was called with the right flags.
	if len(runner.calls) != 1 {
		t.Fatalf("expected 1 kubectl call, got %d", len(runner.calls))
	}
	call := runner.calls[0]
	if call.name != "kubectl" {
		t.Errorf("expected kubectl, got %q", call.name)
	}
	if !containsAll(call.args, []string{"--kubeconfig", kubeconfigPath, "get", "persistentvolumes", "-o", "json"}) {
		t.Errorf("unexpected kubectl args: %v", call.args)
	}
}

func TestCollectCSIVolumeHandles_NoCSIVolumes(t *testing.T) {
	dir := t.TempDir()
	kubeconfigPath := filepath.Join(dir, "kubeconfig.yaml")
	if err := os.WriteFile(kubeconfigPath, []byte("placeholder"), 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}

	// PVs with a different CSI driver (not Cinder) and one non-CSI PV.
	pvJSON := []byte(`{"items":[
		{"spec":{"csi":{"driver":"ebs.csi.aws.com","volumeHandle":"aws-vol-1"}}},
		{"spec":{"hostPath":{"path":"/tmp"}}}
	]}`)
	runner := &fakeCleanupRunner{kubectlOut: pvJSON}

	handles, err := collectCSIVolumeHandles(context.Background(), kubeconfigPath, runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(handles) != 0 {
		t.Errorf("expected no handles for non-Cinder volumes, got %v", handles)
	}
}

func TestCollectCSIVolumeHandles_MissingKubeconfig(t *testing.T) {
	runner := &fakeCleanupRunner{}
	_, err := collectCSIVolumeHandles(context.Background(), "/nonexistent/kubeconfig.yaml", runner)
	if err == nil {
		t.Fatal("expected error for missing kubeconfig")
	}
	if len(runner.calls) != 0 {
		t.Error("kubectl should not be called when kubeconfig is missing")
	}
}

func TestCollectCSIVolumeHandles_EmptyPath(t *testing.T) {
	runner := &fakeCleanupRunner{}
	_, err := collectCSIVolumeHandles(context.Background(), "", runner)
	if err == nil {
		t.Fatal("expected error for empty kubeconfig path")
	}
}

func TestCollectCSIVolumeHandles_KubectlFails(t *testing.T) {
	dir := t.TempDir()
	kubeconfigPath := filepath.Join(dir, "kubeconfig.yaml")
	if err := os.WriteFile(kubeconfigPath, []byte("placeholder"), 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}

	runner := &fakeCleanupRunner{kubectlErr: fmt.Errorf("connection refused")}
	_, err := collectCSIVolumeHandles(context.Background(), kubeconfigPath, runner)
	if err == nil {
		t.Fatal("expected error when kubectl fails")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// ---------------------------------------------------------------------------
// cleanupCSIVolumes tests
// ---------------------------------------------------------------------------

func TestCleanupCSIVolumes_EmptyHandles(t *testing.T) {
	svc := &fakeCinderSvc{}
	buf := &bytes.Buffer{}
	if err := cleanupCSIVolumes(context.Background(), nil, svc, false, buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "No orphaned") {
		t.Errorf("expected 'No orphaned' message, got: %s", buf.String())
	}
	if len(svc.deleted) != 0 {
		t.Errorf("expected no deletions, got: %v", svc.deleted)
	}
}

func TestCleanupCSIVolumes_ReportOnly(t *testing.T) {
	svc := &fakeCinderSvc{
		volumes: map[string]struct {
			status string
			size   int
		}{
			"vol-aaa": {status: "available", size: 10},
			"vol-bbb": {status: "available", size: 50},
		},
	}
	buf := &bytes.Buffer{}
	if err := cleanupCSIVolumes(context.Background(), []string{"vol-aaa", "vol-bbb"}, svc, false, buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Nothing should be deleted in report-only mode.
	if len(svc.deleted) != 0 {
		t.Errorf("expected no deletions in report mode, got: %v", svc.deleted)
	}

	output := buf.String()
	if !strings.Contains(output, "Warning:") {
		t.Errorf("expected Warning in output, got: %s", output)
	}
	if !strings.Contains(output, "vol-aaa") {
		t.Errorf("expected vol-aaa in output, got: %s", output)
	}
	if !strings.Contains(output, "vol-bbb") {
		t.Errorf("expected vol-bbb in output, got: %s", output)
	}
	if !strings.Contains(output, "60 GB") {
		t.Errorf("expected total GB in output, got: %s", output)
	}
	// Should suggest the manual delete command.
	if !strings.Contains(output, "openstack volume delete") {
		t.Errorf("expected manual delete command hint, got: %s", output)
	}
}

func TestCleanupCSIVolumes_DeleteAvailable(t *testing.T) {
	svc := &fakeCinderSvc{
		volumes: map[string]struct {
			status string
			size   int
		}{
			"vol-111": {status: "available", size: 10},
			"vol-222": {status: "available", size: 20},
		},
	}
	buf := &bytes.Buffer{}
	if err := cleanupCSIVolumes(context.Background(), []string{"vol-111", "vol-222"}, svc, true, buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(svc.deleted) != 2 {
		t.Fatalf("expected 2 deletions, got %v", svc.deleted)
	}
	deleted := map[string]bool{}
	for _, d := range svc.deleted {
		deleted[d] = true
	}
	for _, want := range []string{"vol-111", "vol-222"} {
		if !deleted[want] {
			t.Errorf("expected %q deleted, got: %v", want, svc.deleted)
		}
	}

	output := buf.String()
	if !strings.Contains(output, "Successfully deleted") {
		t.Errorf("expected success message, got: %s", output)
	}
	if !strings.Contains(output, "30 GB") {
		t.Errorf("expected total GB freed in output, got: %s", output)
	}
}

// TestCleanupCSIVolumes_SkipNonAvailable asserts that volumes not in the
// "available" state are never deleted regardless of the deleteVolumes flag.
func TestCleanupCSIVolumes_SkipNonAvailable(t *testing.T) {
	svc := &fakeCinderSvc{
		volumes: map[string]struct {
			status string
			size   int
		}{
			"vol-available": {status: "available", size: 10},
			"vol-in-use":    {status: "in-use", size: 20},
			"vol-reserved":  {status: "reserved", size: 30},
		},
	}
	buf := &bytes.Buffer{}
	if err := cleanupCSIVolumes(context.Background(),
		[]string{"vol-available", "vol-in-use", "vol-reserved"}, svc, true, buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(svc.deleted) != 1 || svc.deleted[0] != "vol-available" {
		t.Errorf("expected only vol-available deleted, got: %v", svc.deleted)
	}

	output := buf.String()
	if !strings.Contains(output, "in-use") {
		t.Errorf("expected in-use status in skipped output, got: %s", output)
	}
	if !strings.Contains(output, "reserved") {
		t.Errorf("expected reserved status in skipped output, got: %s", output)
	}
}

// TestCleanupCSIVolumes_OutOfScopeHandlesNeverTouched asserts that only
// handles in the provided list are considered — volumes outside the list are
// never queried or deleted.
func TestCleanupCSIVolumes_OutOfScopeHandlesNeverTouched(t *testing.T) {
	svc := &fakeCinderSvc{
		volumes: map[string]struct {
			status string
			size   int
		}{
			"vol-in-scope":  {status: "available", size: 5},
			"vol-out-scope": {status: "available", size: 5},
		},
	}
	buf := &bytes.Buffer{}
	// Only pass the in-scope handle.
	if err := cleanupCSIVolumes(context.Background(), []string{"vol-in-scope"}, svc, true, buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(svc.deleted) != 1 || svc.deleted[0] != "vol-in-scope" {
		t.Errorf("expected only in-scope volume deleted, got: %v", svc.deleted)
	}
}

func TestCleanupCSIVolumes_AlreadyGoneVolume(t *testing.T) {
	// Simulate a volume that was already deleted (GetStatus returns an error).
	svc := &fakeCinderSvc{
		volumes: map[string]struct {
			status string
			size   int
		}{
			"vol-exists": {status: "available", size: 10},
			// vol-gone is not in the map → GetStatus returns "not found"
		},
	}
	buf := &bytes.Buffer{}
	if err := cleanupCSIVolumes(context.Background(),
		[]string{"vol-exists", "vol-gone"}, svc, true, buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// vol-gone returns "not found" from GetStatus; it should be skipped as already deleted.
	if len(svc.deleted) != 1 || svc.deleted[0] != "vol-exists" {
		t.Errorf("expected only vol-exists deleted, got: %v", svc.deleted)
	}
	if !strings.Contains(buf.String(), "already deleted") {
		t.Errorf("expected 'already deleted' for vol-gone, got: %s", buf.String())
	}
}

// TestCleanupCSIVolumes_DeleteFailureIsReported asserts that when a delete fails
// (not a 404), cleanupCSIVolumes returns an error aggregating the failures.
func TestCleanupCSIVolumes_DeleteFailureIsReported(t *testing.T) {
	svc := &fakeCinderSvc{
		volumes: map[string]struct {
			status string
			size   int
		}{
			"vol-ok":   {status: "available", size: 10},
			"vol-fail": {status: "available", size: 20},
		},
		deleteErr: map[string]error{
			"vol-fail": fmt.Errorf("API error: 500 internal server error"),
		},
	}
	buf := &bytes.Buffer{}
	err := cleanupCSIVolumes(context.Background(),
		[]string{"vol-ok", "vol-fail"}, svc, true, buf)

	// Should return an aggregated error listing the failures.
	if err == nil {
		t.Fatal("expected error when delete fails, got nil")
	}
	if !strings.Contains(err.Error(), "failed to delete") {
		t.Errorf("expected 'failed to delete' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "vol-fail") {
		t.Errorf("expected vol-fail in error, got: %v", err)
	}

	// vol-ok should be deleted despite vol-fail error.
	if len(svc.deleted) != 1 || svc.deleted[0] != "vol-ok" {
		t.Errorf("expected vol-ok deleted despite failure, got: %v", svc.deleted)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// containsAll returns true if slice contains every string in want.
func containsAll(slice, want []string) bool {
	set := make(map[string]bool, len(slice))
	for _, s := range slice {
		set[s] = true
	}
	for _, w := range want {
		if !set[w] {
			return false
		}
	}
	return true
}
