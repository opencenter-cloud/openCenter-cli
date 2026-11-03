// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
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

package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestClusterReconcileCommandSurface(t *testing.T) {
	root := newGARootForCommandSurfaceTest()
	reconcileCmd := requireCommandPath(t, root, "cluster", "reconcile")

	expectedFlags := []string{
		"service",
		"all",
		"message",
		"remote",
		"push-only",
		"skip-flux",
		"flux-only",
		"generate",
		"wait",
		"timeout",
		"kubeconfig",
	}

	for _, flag := range expectedFlags {
		if reconcileCmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected flag --%s on 'cluster reconcile'", flag)
		}
	}
}

func TestClusterServiceReconcileCommandSurface(t *testing.T) {
	root := newGARootForCommandSurfaceTest()
	reconcileCmd := requireCommandPath(t, root, "cluster", "service", "reconcile")

	expectedFlags := []string{
		"cluster",
		"message",
		"remote",
		"push-only",
		"skip-flux",
		"flux-only",
		"generate",
		"wait",
		"timeout",
		"kubeconfig",
	}

	for _, flag := range expectedFlags {
		if reconcileCmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected flag --%s on 'cluster service reconcile'", flag)
		}
	}
}

func TestClusterReconcileMutuallyExclusiveFlags(t *testing.T) {
	root := newGARootForCommandSurfaceTest()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	root.SetArgs([]string{"cluster", "reconcile", "test-cluster", "--push-only", "--flux-only"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when combining --push-only and --flux-only, got nil")
	}
	if !strings.Contains(err.Error(), "--push-only/--skip-flux and --flux-only cannot be used together") {
		t.Errorf("unexpected error message: %v", err)
	}
}
