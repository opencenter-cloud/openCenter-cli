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
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/rackerlabs/opencenter-cli/internal/cluster"
	"github.com/rackerlabs/opencenter-cli/internal/di"
)

// HotPathAnalysis contains analysis results for hot paths
type HotPathAnalysis struct {
	TotalDuration      time.Duration
	CPUHotPaths        []HotPath
	MemoryHotPaths     []HotPath
	AllocationHotPaths []HotPath
	BlockingOperations []HotPath
}

// HotPath represents a hot path in the code
type HotPath struct {
	Function    string
	Percentage  float64
	Cumulative  float64
	SelfTime    string
	TotalTime   string
	Allocations string
}

// TestClusterInitHotPathAnalysis performs comprehensive hot path analysis
func TestClusterInitHotPathAnalysis(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping hot path analysis in short mode")
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

	// Run comprehensive profiling
	analysis, err := runComprehensiveProfiling(t, tmpDir, profileDir)
	if err != nil {
		t.Fatalf("Failed to run profiling: %v", err)
	}

	// Display analysis results
	displayAnalysisResults(t, analysis)

	// Save analysis to file
	analysisPath := filepath.Join(profileDir, "hotpath_analysis.txt")
	if err := saveAnalysisToFile(analysis, analysisPath); err != nil {
		t.Logf("Warning: Failed to save analysis to file: %v", err)
	} else {
		t.Logf("\nHot path analysis saved to: %s", analysisPath)
	}
}

// runComprehensiveProfiling runs all profiling types and collects data
func runComprehensiveProfiling(t *testing.T, tmpDir, profileDir string) (*HotPathAnalysis, error) {
	analysis := &HotPathAnalysis{}

	// 1. CPU Profiling
	cpuProfilePath := filepath.Join(profileDir, "cpu_profile.prof")
	cpuFile, err := os.Create(cpuProfilePath)
	if err != nil {
		return nil, fmt.Errorf("creating CPU profile file: %w", err)
	}

	if err := pprof.StartCPUProfile(cpuFile); err != nil {
		cpuFile.Close()
		return nil, fmt.Errorf("starting CPU profiling: %w", err)
	}

	// Run initialization with CPU profiling
	startTime := time.Now()
	if err := runClusterInitForProfiling(t, tmpDir); err != nil {
		pprof.StopCPUProfile()
		cpuFile.Close()
		return nil, fmt.Errorf("cluster initialization failed: %w", err)
	}
	analysis.TotalDuration = time.Since(startTime)

	pprof.StopCPUProfile()
	cpuFile.Close()

	// Analyze CPU profile
	cpuHotPaths, err := analyzeCPUProfile(cpuProfilePath)
	if err != nil {
		t.Logf("Warning: Failed to analyze CPU profile: %v", err)
	} else {
		analysis.CPUHotPaths = cpuHotPaths
	}

	// 2. Memory Profiling
	runtime.GC() // Force GC before memory profiling

	memProfilePath := filepath.Join(profileDir, "mem_profile.prof")
	memFile, err := os.Create(memProfilePath)
	if err != nil {
		return nil, fmt.Errorf("creating memory profile file: %w", err)
	}
	defer memFile.Close()

	if err := pprof.WriteHeapProfile(memFile); err != nil {
		return nil, fmt.Errorf("writing heap profile: %w", err)
	}

	// Analyze memory profile
	memHotPaths, err := analyzeMemoryProfile(memProfilePath)
	if err != nil {
		t.Logf("Warning: Failed to analyze memory profile: %v", err)
	} else {
		analysis.MemoryHotPaths = memHotPaths
	}

	// 3. Allocation Profiling
	allocHotPaths, err := analyzeAllocationProfile(memProfilePath)
	if err != nil {
		t.Logf("Warning: Failed to analyze allocation profile: %v", err)
	} else {
		analysis.AllocationHotPaths = allocHotPaths
	}

	return analysis, nil
}

// analyzeCPUProfile analyzes CPU profile and extracts hot paths
func analyzeCPUProfile(profilePath string) ([]HotPath, error) {
	// Use go tool pprof to analyze the profile
	cmd := exec.Command("go", "tool", "pprof", "-top", "-cum", profilePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("running pprof: %w (output: %s)", err, string(output))
	}

	return parseProfileOutput(string(output)), nil
}

// analyzeMemoryProfile analyzes memory profile and extracts hot paths
func analyzeMemoryProfile(profilePath string) ([]HotPath, error) {
	cmd := exec.Command("go", "tool", "pprof", "-top", "-inuse_space", profilePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("running pprof: %w (output: %s)", err, string(output))
	}

	return parseProfileOutput(string(output)), nil
}

// analyzeAllocationProfile analyzes allocation profile and extracts hot paths
func analyzeAllocationProfile(profilePath string) ([]HotPath, error) {
	cmd := exec.Command("go", "tool", "pprof", "-top", "-alloc_space", profilePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("running pprof: %w (output: %s)", err, string(output))
	}

	return parseProfileOutput(string(output)), nil
}

// parseProfileOutput parses pprof output and extracts hot paths
func parseProfileOutput(output string) []HotPath {
	var hotPaths []HotPath
	lines := strings.Split(output, "\n")

	// Skip header lines
	dataStarted := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Look for the data section (starts after header)
		if strings.Contains(line, "flat") && strings.Contains(line, "cum") {
			dataStarted = true
			continue
		}

		if !dataStarted {
			continue
		}

		// Parse data lines
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}

		// Skip lines that don't start with a number
		if !strings.Contains(fields[0], ".") && !strings.Contains(fields[0], "%") {
			continue
		}

		hotPath := HotPath{
			SelfTime:   fields[0],
			Percentage: parsePercentage(fields[1]),
			Cumulative: parsePercentage(fields[3]),
			TotalTime:  fields[2],
			Function:   strings.Join(fields[4:], " "),
		}

		hotPaths = append(hotPaths, hotPath)

		// Limit to top 10
		if len(hotPaths) >= 10 {
			break
		}
	}

	return hotPaths
}

// parsePercentage parses percentage string to float64
func parsePercentage(s string) float64 {
	s = strings.TrimSuffix(s, "%")
	var pct float64
	fmt.Sscanf(s, "%f", &pct)
	return pct
}

// displayAnalysisResults displays the analysis results
func displayAnalysisResults(t *testing.T, analysis *HotPathAnalysis) {
	separator := strings.Repeat("=", 80)
	dashedLine := strings.Repeat("-", 80)

	t.Logf("\n%s", separator)
	t.Logf("CLUSTER INITIALIZATION HOT PATH ANALYSIS")
	t.Logf("%s", separator)
	t.Logf("\nTotal Duration: %v", analysis.TotalDuration)

	if len(analysis.CPUHotPaths) > 0 {
		t.Logf("\n%s", dashedLine)
		t.Logf("CPU HOT PATHS (Top 10)")
		t.Logf("%s", dashedLine)
		t.Logf("%-10s %-10s %-10s %-10s %s", "SELF", "SELF%", "CUM", "CUM%", "FUNCTION")
		t.Logf("%s", dashedLine)
		for _, hp := range analysis.CPUHotPaths {
			t.Logf("%-10s %-10.2f%% %-10s %-10.2f%% %s",
				hp.SelfTime, hp.Percentage, hp.TotalTime, hp.Cumulative, hp.Function)
		}
	}

	if len(analysis.MemoryHotPaths) > 0 {
		t.Logf("\n%s", dashedLine)
		t.Logf("MEMORY HOT PATHS (Top 10 - In-Use Space)")
		t.Logf("%s", dashedLine)
		t.Logf("%-10s %-10s %-10s %-10s %s", "SELF", "SELF%", "CUM", "CUM%", "FUNCTION")
		t.Logf("%s", dashedLine)
		for _, hp := range analysis.MemoryHotPaths {
			t.Logf("%-10s %-10.2f%% %-10s %-10.2f%% %s",
				hp.SelfTime, hp.Percentage, hp.TotalTime, hp.Cumulative, hp.Function)
		}
	}

	if len(analysis.AllocationHotPaths) > 0 {
		t.Logf("\n%s", dashedLine)
		t.Logf("ALLOCATION HOT PATHS (Top 10 - Total Allocated)")
		t.Logf("%s", dashedLine)
		t.Logf("%-10s %-10s %-10s %-10s %s", "SELF", "SELF%", "CUM", "CUM%", "FUNCTION")
		t.Logf("%s", dashedLine)
		for _, hp := range analysis.AllocationHotPaths {
			t.Logf("%-10s %-10.2f%% %-10s %-10.2f%% %s",
				hp.SelfTime, hp.Percentage, hp.TotalTime, hp.Cumulative, hp.Function)
		}
	}

	t.Logf("\n%s", separator)
	t.Logf("KEY FINDINGS")
	t.Logf("%s", separator)

	findings := generateKeyFindings(analysis)
	for i, finding := range findings {
		t.Logf("%d. %s", i+1, finding)
	}
}

// generateKeyFindings generates key findings from the analysis
func generateKeyFindings(analysis *HotPathAnalysis) []string {
	var findings []string

	// Analyze CPU hot paths
	if len(analysis.CPUHotPaths) > 0 {
		topCPU := analysis.CPUHotPaths[0]
		if topCPU.Percentage > 10.0 {
			findings = append(findings, fmt.Sprintf(
				"CPU bottleneck detected: %s consumes %.2f%% of CPU time",
				topCPU.Function, topCPU.Percentage))
		}

		// Check for YAML-related operations
		for _, hp := range analysis.CPUHotPaths {
			if strings.Contains(hp.Function, "yaml") || strings.Contains(hp.Function, "Unmarshal") {
				findings = append(findings, fmt.Sprintf(
					"YAML parsing is a hot path: %s (%.2f%%)",
					hp.Function, hp.Percentage))
				break
			}
		}

		// Check for reflection operations
		for _, hp := range analysis.CPUHotPaths {
			if strings.Contains(hp.Function, "reflect") {
				findings = append(findings, fmt.Sprintf(
					"Reflection overhead detected: %s (%.2f%%)",
					hp.Function, hp.Percentage))
				break
			}
		}
	}

	// Analyze memory hot paths
	if len(analysis.MemoryHotPaths) > 0 {
		topMem := analysis.MemoryHotPaths[0]
		if topMem.Percentage > 15.0 {
			findings = append(findings, fmt.Sprintf(
				"Memory allocation hotspot: %s allocates %.2f%% of memory",
				topMem.Function, topMem.Percentage))
		}
	}

	// Performance assessment
	if analysis.TotalDuration < 10*time.Millisecond {
		findings = append(findings, "Excellent performance: initialization completes in <10ms")
	} else if analysis.TotalDuration < 50*time.Millisecond {
		findings = append(findings, "Good performance: initialization completes in <50ms")
	} else if analysis.TotalDuration < 100*time.Millisecond {
		findings = append(findings, "Acceptable performance: initialization completes in <100ms")
	} else {
		findings = append(findings, fmt.Sprintf(
			"Performance concern: initialization takes %v (target: <100ms)",
			analysis.TotalDuration))
	}

	if len(findings) == 0 {
		findings = append(findings, "No significant performance bottlenecks detected")
	}

	return findings
}

// saveAnalysisToFile saves the analysis results to a file
func saveAnalysisToFile(analysis *HotPathAnalysis, path string) error {
	var buf bytes.Buffer

	buf.WriteString(strings.Repeat("=", 80) + "\n")
	buf.WriteString("CLUSTER INITIALIZATION HOT PATH ANALYSIS\n")
	buf.WriteString(strings.Repeat("=", 80) + "\n")
	buf.WriteString(fmt.Sprintf("\nTotal Duration: %v\n", analysis.TotalDuration))

	if len(analysis.CPUHotPaths) > 0 {
		buf.WriteString("\n" + strings.Repeat("-", 80) + "\n")
		buf.WriteString("CPU HOT PATHS (Top 10)\n")
		buf.WriteString(strings.Repeat("-", 80) + "\n")
		buf.WriteString(fmt.Sprintf("%-10s %-10s %-10s %-10s %s\n", "SELF", "SELF%", "CUM", "CUM%", "FUNCTION"))
		buf.WriteString(strings.Repeat("-", 80) + "\n")
		for _, hp := range analysis.CPUHotPaths {
			buf.WriteString(fmt.Sprintf("%-10s %-10.2f%% %-10s %-10.2f%% %s\n",
				hp.SelfTime, hp.Percentage, hp.TotalTime, hp.Cumulative, hp.Function))
		}
	}

	if len(analysis.MemoryHotPaths) > 0 {
		buf.WriteString("\n" + strings.Repeat("-", 80) + "\n")
		buf.WriteString("MEMORY HOT PATHS (Top 10 - In-Use Space)\n")
		buf.WriteString(strings.Repeat("-", 80) + "\n")
		buf.WriteString(fmt.Sprintf("%-10s %-10s %-10s %-10s %s\n", "SELF", "SELF%", "CUM", "CUM%", "FUNCTION"))
		buf.WriteString(strings.Repeat("-", 80) + "\n")
		for _, hp := range analysis.MemoryHotPaths {
			buf.WriteString(fmt.Sprintf("%-10s %-10.2f%% %-10s %-10.2f%% %s\n",
				hp.SelfTime, hp.Percentage, hp.TotalTime, hp.Cumulative, hp.Function))
		}
	}

	if len(analysis.AllocationHotPaths) > 0 {
		buf.WriteString("\n" + strings.Repeat("-", 80) + "\n")
		buf.WriteString("ALLOCATION HOT PATHS (Top 10 - Total Allocated)\n")
		buf.WriteString(strings.Repeat("-", 80) + "\n")
		buf.WriteString(fmt.Sprintf("%-10s %-10s %-10s %-10s %s\n", "SELF", "SELF%", "CUM", "CUM%", "FUNCTION"))
		buf.WriteString(strings.Repeat("-", 80) + "\n")
		for _, hp := range analysis.AllocationHotPaths {
			buf.WriteString(fmt.Sprintf("%-10s %-10.2f%% %-10s %-10.2f%% %s\n",
				hp.SelfTime, hp.Percentage, hp.TotalTime, hp.Cumulative, hp.Function))
		}
	}

	buf.WriteString("\n" + strings.Repeat("=", 80) + "\n")
	buf.WriteString("KEY FINDINGS\n")
	buf.WriteString(strings.Repeat("=", 80) + "\n")

	findings := generateKeyFindings(analysis)
	for i, finding := range findings {
		buf.WriteString(fmt.Sprintf("%d. %s\n", i+1, finding))
	}

	// Add recommendations
	buf.WriteString("\n" + strings.Repeat("=", 80) + "\n")
	buf.WriteString("OPTIMIZATION RECOMMENDATIONS\n")
	buf.WriteString(strings.Repeat("=", 80) + "\n")
	recommendations := generateRecommendations(analysis)
	for i, rec := range recommendations {
		buf.WriteString(fmt.Sprintf("%d. %s\n", i+1, rec))
	}

	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// generateRecommendations generates optimization recommendations
func generateRecommendations(analysis *HotPathAnalysis) []string {
	var recommendations []string

	// Check for YAML parsing overhead
	hasYAMLOverhead := false
	for _, hp := range analysis.CPUHotPaths {
		if strings.Contains(hp.Function, "yaml") || strings.Contains(hp.Function, "Unmarshal") {
			hasYAMLOverhead = true
			break
		}
	}
	if hasYAMLOverhead {
		recommendations = append(recommendations,
			"Consider caching parsed YAML configurations to reduce parsing overhead")
	}

	// Check for reflection overhead
	hasReflectionOverhead := false
	for _, hp := range analysis.CPUHotPaths {
		if strings.Contains(hp.Function, "reflect") {
			hasReflectionOverhead = true
			break
		}
	}
	if hasReflectionOverhead {
		recommendations = append(recommendations,
			"Reduce reflection usage by using code generation or type-specific handlers")
	}

	// Check for memory allocations
	if len(analysis.AllocationHotPaths) > 0 {
		topAlloc := analysis.AllocationHotPaths[0]
		if topAlloc.Percentage > 20.0 {
			recommendations = append(recommendations,
				fmt.Sprintf("Optimize memory allocations in %s (%.2f%% of total)",
					topAlloc.Function, topAlloc.Percentage))
		}
	}

	// Performance-based recommendations
	if analysis.TotalDuration > 100*time.Millisecond {
		recommendations = append(recommendations,
			"Consider implementing lazy initialization for non-critical components")
		recommendations = append(recommendations,
			"Profile individual operations to identify specific bottlenecks")
	}

	// General recommendations
	recommendations = append(recommendations,
		"Implement connection pooling for frequently accessed resources")
	recommendations = append(recommendations,
		"Use sync.Pool for frequently allocated temporary objects")
	recommendations = append(recommendations,
		"Consider parallel initialization of independent components")

	return recommendations
}

// TestClusterInitPerformanceRegression tests for performance regressions
func TestClusterInitPerformanceRegression(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance regression test in short mode")
	}

	// Create temporary directory for test cluster
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Unsetenv("HOME")

	// Run multiple iterations to get average
	const iterations = 10
	durations := make([]time.Duration, iterations)

	for i := 0; i < iterations; i++ {
		ctx := context.Background()
		container := di.NewContainer()
		if err := setupContainer(container); err != nil {
			t.Fatalf("Setting up DI container: %v", err)
		}

		var initService *cluster.InitService
		if err := container.ResolveAs("init-service", &initService); err != nil {
			t.Fatalf("Resolving init service: %v", err)
		}

		opts := cluster.InitOptions{
			ClusterName:  fmt.Sprintf("perf-test-%d", i),
			Organization: "opencenter",
			Provider:     "openstack",
			NoKeyGen:     true,
			NoGitInit:    true,
			Force:        true,
		}

		start := time.Now()
		if _, err := initService.Initialize(ctx, opts); err != nil {
			t.Fatalf("Initialization failed: %v", err)
		}
		durations[i] = time.Since(start)
	}

	// Calculate statistics
	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})

	var total time.Duration
	for _, d := range durations {
		total += d
	}
	avg := total / time.Duration(iterations)
	median := durations[iterations/2]
	p95Index := int(math.Round(float64(iterations) * 0.95))
	if p95Index >= iterations {
		p95Index = iterations - 1
	}
	p95 := durations[p95Index]

	t.Logf("\n%s", strings.Repeat("=", 80))
	t.Logf("PERFORMANCE REGRESSION TEST (%d iterations)", iterations)
	t.Logf("%s", strings.Repeat("=", 80))
	t.Logf("Average:  %v", avg)
	t.Logf("Median:   %v", median)
	t.Logf("P95:      %v", p95)
	t.Logf("Min:      %v", durations[0])
	t.Logf("Max:      %v", durations[iterations-1])

	// Performance targets from design document
	const targetAvg = 100 * time.Millisecond
	const targetP95 = 150 * time.Millisecond

	if avg > targetAvg {
		t.Logf("WARNING: Average initialization time (%v) exceeds target (%v)", avg, targetAvg)
	} else {
		t.Logf("✓ Average initialization time within target")
	}

	if p95 > targetP95 {
		t.Logf("WARNING: P95 initialization time (%v) exceeds target (%v)", p95, targetP95)
	} else {
		t.Logf("✓ P95 initialization time within target")
	}
}
