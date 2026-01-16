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

// Package metrics provides performance tracking and metrics collection for openCenter operations.
// It tracks template rendering times, configuration building times, and GitOps generation times.
package metrics

import (
	"fmt"
	"sync"
	"time"
)

// MetricType represents the type of metric being tracked.
type MetricType string

const (
	// MetricTypeTemplateRender tracks template rendering operations
	MetricTypeTemplateRender MetricType = "template_render"

	// MetricTypeConfigBuild tracks configuration building operations
	MetricTypeConfigBuild MetricType = "config_build"

	// MetricTypeGitOpsGeneration tracks GitOps repository generation operations
	MetricTypeGitOpsGeneration MetricType = "gitops_generation"

	// MetricTypeValidation tracks validation operations
	MetricTypeValidation MetricType = "validation"

	// MetricTypeMigration tracks configuration migration operations
	MetricTypeMigration MetricType = "migration"
)

// Metric represents a single performance metric measurement.
type Metric struct {
	// Type is the type of operation being measured
	Type MetricType

	// Name is a descriptive name for this specific metric
	Name string

	// Duration is how long the operation took
	Duration time.Duration

	// Timestamp is when the metric was recorded
	Timestamp time.Time

	// Metadata contains additional context about the metric
	Metadata map[string]interface{}

	// Success indicates whether the operation completed successfully
	Success bool

	// Error contains any error that occurred during the operation
	Error error
}

// MetricsCollector collects and aggregates performance metrics.
type MetricsCollector struct {
	mu      sync.RWMutex
	metrics []Metric
	enabled bool
}

// NewMetricsCollector creates a new metrics collector.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		metrics: make([]Metric, 0),
		enabled: true,
	}
}

// SetEnabled enables or disables metrics collection.
func (mc *MetricsCollector) SetEnabled(enabled bool) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.enabled = enabled
}

// IsEnabled returns whether metrics collection is enabled.
func (mc *MetricsCollector) IsEnabled() bool {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.enabled
}

// RecordMetric records a single metric.
func (mc *MetricsCollector) RecordMetric(metric Metric) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if !mc.enabled {
		return
	}

	// Set timestamp if not already set
	if metric.Timestamp.IsZero() {
		metric.Timestamp = time.Now()
	}

	mc.metrics = append(mc.metrics, metric)
}

// GetMetrics returns all collected metrics.
func (mc *MetricsCollector) GetMetrics() []Metric {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make([]Metric, len(mc.metrics))
	copy(result, mc.metrics)
	return result
}

// GetMetricsByType returns all metrics of a specific type.
func (mc *MetricsCollector) GetMetricsByType(metricType MetricType) []Metric {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	var result []Metric
	for _, m := range mc.metrics {
		if m.Type == metricType {
			result = append(result, m)
		}
	}
	return result
}

// Clear clears all collected metrics.
func (mc *MetricsCollector) Clear() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.metrics = make([]Metric, 0)
}

// GetSummary returns a summary of collected metrics.
func (mc *MetricsCollector) GetSummary() MetricsSummary {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	summary := MetricsSummary{
		TotalMetrics: len(mc.metrics),
		ByType:       make(map[MetricType]TypeSummary),
	}

	// Group metrics by type
	typeMetrics := make(map[MetricType][]Metric)
	for _, m := range mc.metrics {
		typeMetrics[m.Type] = append(typeMetrics[m.Type], m)
	}

	// Calculate statistics for each type
	for metricType, metrics := range typeMetrics {
		summary.ByType[metricType] = calculateTypeSummary(metrics)
	}

	return summary
}

// MetricsSummary provides aggregated statistics about collected metrics.
type MetricsSummary struct {
	// TotalMetrics is the total number of metrics collected
	TotalMetrics int

	// ByType contains summaries grouped by metric type
	ByType map[MetricType]TypeSummary
}

// TypeSummary contains statistics for a specific metric type.
type TypeSummary struct {
	// Count is the number of metrics of this type
	Count int

	// SuccessCount is the number of successful operations
	SuccessCount int

	// FailureCount is the number of failed operations
	FailureCount int

	// TotalDuration is the sum of all durations
	TotalDuration time.Duration

	// AverageDuration is the average duration
	AverageDuration time.Duration

	// MinDuration is the minimum duration
	MinDuration time.Duration

	// MaxDuration is the maximum duration
	MaxDuration time.Duration

	// P50Duration is the 50th percentile (median) duration
	P50Duration time.Duration

	// P95Duration is the 95th percentile duration
	P95Duration time.Duration

	// P99Duration is the 99th percentile duration
	P99Duration time.Duration
}

// calculateTypeSummary calculates statistics for a set of metrics.
func calculateTypeSummary(metrics []Metric) TypeSummary {
	if len(metrics) == 0 {
		return TypeSummary{}
	}

	summary := TypeSummary{
		Count:       len(metrics),
		MinDuration: metrics[0].Duration,
		MaxDuration: metrics[0].Duration,
	}

	// Calculate basic statistics
	var totalDuration time.Duration
	for _, m := range metrics {
		totalDuration += m.Duration

		if m.Success {
			summary.SuccessCount++
		} else {
			summary.FailureCount++
		}

		if m.Duration < summary.MinDuration {
			summary.MinDuration = m.Duration
		}
		if m.Duration > summary.MaxDuration {
			summary.MaxDuration = m.Duration
		}
	}

	summary.TotalDuration = totalDuration
	summary.AverageDuration = totalDuration / time.Duration(len(metrics))

	// Calculate percentiles
	durations := make([]time.Duration, len(metrics))
	for i, m := range metrics {
		durations[i] = m.Duration
	}
	sortDurations(durations)

	summary.P50Duration = percentile(durations, 50)
	summary.P95Duration = percentile(durations, 95)
	summary.P99Duration = percentile(durations, 99)

	return summary
}

// sortDurations sorts a slice of durations in ascending order.
func sortDurations(durations []time.Duration) {
	// Simple bubble sort (sufficient for small datasets)
	n := len(durations)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if durations[j] > durations[j+1] {
				durations[j], durations[j+1] = durations[j+1], durations[j]
			}
		}
	}
}

// percentile calculates the nth percentile of a sorted slice of durations.
func percentile(sortedDurations []time.Duration, p int) time.Duration {
	if len(sortedDurations) == 0 {
		return 0
	}

	// Calculate index for percentile
	index := (p * len(sortedDurations)) / 100
	if index >= len(sortedDurations) {
		index = len(sortedDurations) - 1
	}

	return sortedDurations[index]
}

// String returns a human-readable string representation of the summary.
func (s MetricsSummary) String() string {
	result := fmt.Sprintf("Metrics Summary (Total: %d)\n", s.TotalMetrics)
	result += "================================\n\n"

	for metricType, typeSummary := range s.ByType {
		result += fmt.Sprintf("%s:\n", metricType)
		result += fmt.Sprintf("  Count: %d (Success: %d, Failure: %d)\n",
			typeSummary.Count, typeSummary.SuccessCount, typeSummary.FailureCount)
		result += fmt.Sprintf("  Total Duration: %s\n", typeSummary.TotalDuration)
		result += fmt.Sprintf("  Average Duration: %s\n", typeSummary.AverageDuration)
		result += fmt.Sprintf("  Min Duration: %s\n", typeSummary.MinDuration)
		result += fmt.Sprintf("  Max Duration: %s\n", typeSummary.MaxDuration)
		result += fmt.Sprintf("  P50 Duration: %s\n", typeSummary.P50Duration)
		result += fmt.Sprintf("  P95 Duration: %s\n", typeSummary.P95Duration)
		result += fmt.Sprintf("  P99 Duration: %s\n", typeSummary.P99Duration)
		result += "\n"
	}

	return result
}

// Timer provides a convenient way to measure operation duration.
type Timer struct {
	startTime  time.Time
	collector  *MetricsCollector
	metricType MetricType
	name       string
	metadata   map[string]interface{}
}

// NewTimer creates a new timer that will record a metric when stopped.
func (mc *MetricsCollector) NewTimer(metricType MetricType, name string) *Timer {
	return &Timer{
		startTime:  time.Now(),
		collector:  mc,
		metricType: metricType,
		name:       name,
		metadata:   make(map[string]interface{}),
	}
}

// WithMetadata adds metadata to the timer.
func (t *Timer) WithMetadata(key string, value interface{}) *Timer {
	t.metadata[key] = value
	return t
}

// Stop stops the timer and records the metric.
func (t *Timer) Stop() time.Duration {
	duration := time.Since(t.startTime)
	t.collector.RecordMetric(Metric{
		Type:      t.metricType,
		Name:      t.name,
		Duration:  duration,
		Timestamp: time.Now(),
		Metadata:  t.metadata,
		Success:   true,
	})
	return duration
}

// StopWithError stops the timer and records the metric with an error.
func (t *Timer) StopWithError(err error) time.Duration {
	duration := time.Since(t.startTime)
	t.collector.RecordMetric(Metric{
		Type:      t.metricType,
		Name:      t.name,
		Duration:  duration,
		Timestamp: time.Now(),
		Metadata:  t.metadata,
		Success:   false,
		Error:     err,
	})
	return duration
}

// Global metrics collector instance
var globalCollector = NewMetricsCollector()

// GetGlobalCollector returns the global metrics collector.
func GetGlobalCollector() *MetricsCollector {
	return globalCollector
}

// RecordTemplateRender records a template rendering metric.
func RecordTemplateRender(templatePath string, duration time.Duration, success bool, err error) {
	globalCollector.RecordMetric(Metric{
		Type:     MetricTypeTemplateRender,
		Name:     templatePath,
		Duration: duration,
		Metadata: map[string]interface{}{
			"template_path": templatePath,
		},
		Success: success,
		Error:   err,
	})
}

// RecordConfigBuild records a configuration building metric.
func RecordConfigBuild(clusterName string, duration time.Duration, success bool, err error) {
	globalCollector.RecordMetric(Metric{
		Type:     MetricTypeConfigBuild,
		Name:     clusterName,
		Duration: duration,
		Metadata: map[string]interface{}{
			"cluster_name": clusterName,
		},
		Success: success,
		Error:   err,
	})
}

// RecordGitOpsGeneration records a GitOps generation metric.
func RecordGitOpsGeneration(clusterName string, duration time.Duration, filesGenerated int, success bool, err error) {
	globalCollector.RecordMetric(Metric{
		Type:     MetricTypeGitOpsGeneration,
		Name:     clusterName,
		Duration: duration,
		Metadata: map[string]interface{}{
			"cluster_name":    clusterName,
			"files_generated": filesGenerated,
		},
		Success: success,
		Error:   err,
	})
}
