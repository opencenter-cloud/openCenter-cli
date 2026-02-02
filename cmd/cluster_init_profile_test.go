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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/rackerlabs/opencenter-cli/internal/cluster"
	"github.com/rackerlabs/opencenter-cli/internal/di"
)

// TestClusterInitCPUProfile runs CPU profiling on cluster initialization
func TestClusterInitCPUProfile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping profiling test in short mode")
	}

	// Create temporary directory for test cluster
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Unsetenv("HOME")

	// Create profile output directory
	profileDir := filepath.Join(tmpDir, "profiles")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatalf("Failed to create profile directory: %v", err)
	}

	// Create CPU profile file
	cpuProfilePath := filepath.Join(profileDir, "cpu_profile.prof")
	cpuFile, err := os.Create(cpuProfilePath)
	if err != nil {
		t.Fatalf("Failed to create CPU profile file: %v", err)
	}
	defer cpuFile.Close()

	// Start CPU profiling
	if err := pprof.StartCPUProfile(cpuFile); err != nil {
		t.Fatalf("Failed to start CPU profiling: %v", err)
	}
	defer pprof.StopCPUProfile()

	// Run cluster initialization
	startTime := time.Now()
	if err := runClusterInitForProfiling(t, tmpDir); err != nil {
		t.Fatalf("Cluster initialization failed: %v", err)
	}
	duration := time.Since(startTime)

	t.Logf("Cluster initialization completed in %v", duration)
	t.Logf("CPU profile saved to: %s", cpuProfilePath)
	t.Logf("\nTo analyze CPU profile, run:")
	t.Logf("  go tool pprof -http=:8080 %s", cpuProfilePath)
	t.Logf("  go tool pprof -top %s", cpuProfilePath)
	t.Logf("  go tool pprof -list=Initialize %s", cpuProfilePath)
}

// TestClusterInitMemoryProfile runs memory profiling on cluster initialization
func TestClusterInitMemoryProfile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping profiling test in short mode")
	}

	// Create temporary directory for test cluster
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Unsetenv("HOME")

	// Create profile output directory
	profileDir := filepath.Join(tmpDir, "profiles")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatalf("Failed to create profile directory: %v", err)
	}

	// Record memory stats before
	var memStatsBefore runtime.MemStats
	runtime.ReadMemStats(&memStatsBefore)

	// Run cluster initialization
	startTime := time.Now()
	if err := runClusterInitForProfiling(t, tmpDir); err != nil {
		t.Fatalf("Cluster initialization failed: %v", err)
	}
	duration := time.Since(startTime)

	// Force garbage collection to get accurate memory stats
	runtime.GC()

	// Record memory stats after
	var memStatsAfter runtime.MemStats
	runtime.ReadMemStats(&memStatsAfter)

	// Create memory profile file
	memProfilePath := filepath.Join(profileDir, "mem_profile.prof")
	memFile, err := os.Create(memProfilePath)
	if err != nil {
		t.Fatalf("Failed to create memory profile file: %v", err)
	}
	defer memFile.Close()

	// Write heap profile
	if err := pprof.WriteHeapProfile(memFile); err != nil {
		t.Fatalf("Failed to write heap profile: %v", err)
	}

	// Calculate memory usage
	allocDiff := memStatsAfter.TotalAlloc - memStatsBefore.TotalAlloc
	heapAllocDiff := memStatsAfter.HeapAlloc - memStatsBefore.HeapAlloc

	t.Logf("Cluster initialization completed in %v", duration)
	t.Logf("Total memory allocated: %s", formatBytes(allocDiff))
	t.Logf("Heap memory allocated: %s", formatBytes(heapAllocDiff))
	t.Logf("Peak heap allocation: %s", formatBytes(memStatsAfter.HeapAlloc))
	t.Logf("Memory profile saved to: %s", memProfilePath)
	t.Logf("\nTo analyze memory profile, run:")
	t.Logf("  go tool pprof -http=:8080 %s", memProfilePath)
	t.Logf("  go tool pprof -top %s", memProfilePath)
	t.Logf("  go tool pprof -alloc_space %s", memProfilePath)
}

// TestClusterInitBlockProfile runs blocking profiling on cluster initialization
func TestClusterInitBlockProfile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping profiling test in short mode")
	}

	// Create temporary directory for test cluster
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Unsetenv("HOME")

	// Create profile output directory
	profileDir := filepath.Join(tmpDir, "profiles")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatalf("Failed to create profile directory: %v", err)
	}

	// Enable block profiling
	runtime.SetBlockProfileRate(1)
	defer runtime.SetBlockProfileRate(0)

	// Run cluster initialization
	startTime := time.Now()
	if err := runClusterInitForProfiling(t, tmpDir); err != nil {
		t.Fatalf("Cluster initialization failed: %v", err)
	}
	duration := time.Since(startTime)

	// Create block profile file
	blockProfilePath := filepath.Join(profileDir, "block_profile.prof")
	blockFile, err := os.Create(blockProfilePath)
	if err != nil {
		t.Fatalf("Failed to create block profile file: %v", err)
	}
	defer blockFile.Close()

	// Write block profile
	if err := pprof.Lookup("block").WriteTo(blockFile, 0); err != nil {
		t.Fatalf("Failed to write block profile: %v", err)
	}

	t.Logf("Cluster initialization completed in %v", duration)
	t.Logf("Block profile saved to: %s", blockProfilePath)
	t.Logf("\nTo analyze block profile, run:")
	t.Logf("  go tool pprof -http=:8080 %s", blockProfilePath)
}

// runClusterInitForProfiling runs cluster initialization for profiling purposes
func runClusterInitForProfiling(t *testing.T, tmpDir string) error {
	ctx := context.Background()

	// Initialize DI container
	container := di.NewContainer()
	if err := setupContainer(container); err != nil {
		return fmt.Errorf("setting up DI container: %w", err)
	}

	// Resolve InitService from container
	var initService *cluster.InitService
	if err := container.ResolveAs("init-service", &initService); err != nil {
		return fmt.Errorf("resolving init service: %w", err)
	}

	// Create initialization options
	opts := cluster.InitOptions{
		ClusterName:  "profile-test-cluster",
		Organization: "opencenter",
		Provider:     "openstack",
		NoKeyGen:     true, // Skip key generation for faster profiling
		NoGitInit:    true, // Skip git init for faster profiling
		Force:        true,
	}

	// Execute initialization
	_, err := initService.Initialize(ctx, opts)
	return err
}

// formatBytes formats bytes into human-readable format
func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
