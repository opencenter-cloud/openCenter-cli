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

package gitops

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProgressReporter_Report(t *testing.T) {
	t.Run("reports progress with stage and message", func(t *testing.T) {
		buf := &bytes.Buffer{}
		reporter := NewProgressReporter(ProgressReporterOptions{
			Writer:    buf,
			UseColors: false, // Disable colors for easier testing
		})

		reporter.Report("initialization", 0, "Starting generation")
		reporter.Report("initialization", 50, "Halfway through")
		reporter.Report("initialization", 100, "Complete")

		output := buf.String()
		assert.Contains(t, output, "Initialization")
		assert.Contains(t, output, "Starting generation")
		assert.Contains(t, output, "Halfway through")
		assert.Contains(t, output, "Complete")
		assert.Contains(t, output, "0%")
		assert.Contains(t, output, "50%")
		assert.Contains(t, output, "100%")
	})

	t.Run("shows stage header when stage changes", func(t *testing.T) {
		buf := &bytes.Buffer{}
		reporter := NewProgressReporter(ProgressReporterOptions{
			Writer:    buf,
			UseColors: false,
		})

		reporter.Report("stage1", 0, "Message 1")
		reporter.Report("stage2", 0, "Message 2")

		output := buf.String()
		assert.Contains(t, output, "Stage1")
		assert.Contains(t, output, "Stage2")
	})

	t.Run("quiet mode suppresses output", func(t *testing.T) {
		buf := &bytes.Buffer{}
		reporter := NewProgressReporter(ProgressReporterOptions{
			Writer: buf,
			Quiet:  true,
		})

		reporter.Report("stage1", 50, "This should not appear")

		output := buf.String()
		assert.Empty(t, output)
	})

	t.Run("verbose mode includes elapsed time", func(t *testing.T) {
		buf := &bytes.Buffer{}
		reporter := NewProgressReporter(ProgressReporterOptions{
			Writer:    buf,
			Verbose:   true,
			UseColors: false,
		})

		reporter.Report("stage1", 50, "Processing")

		output := buf.String()
		assert.Contains(t, output, "elapsed:")
	})
}

func TestProgressReporter_Complete(t *testing.T) {
	t.Run("displays completion message", func(t *testing.T) {
		buf := &bytes.Buffer{}
		reporter := NewProgressReporter(ProgressReporterOptions{
			Writer:    buf,
			UseColors: false,
		})

		duration := 5 * time.Second
		filesGenerated := 42

		reporter.Complete(duration, filesGenerated)

		output := buf.String()
		assert.Contains(t, output, "completed successfully")
		assert.Contains(t, output, "Duration:")
		assert.Contains(t, output, "Files generated: 42")
	})

	t.Run("quiet mode suppresses completion message", func(t *testing.T) {
		buf := &bytes.Buffer{}
		reporter := NewProgressReporter(ProgressReporterOptions{
			Writer: buf,
			Quiet:  true,
		})

		reporter.Complete(5*time.Second, 42)

		output := buf.String()
		assert.Empty(t, output)
	})
}

func TestProgressReporter_Error(t *testing.T) {
	t.Run("displays error message", func(t *testing.T) {
		buf := &bytes.Buffer{}
		reporter := NewProgressReporter(ProgressReporterOptions{
			Writer:    buf,
			UseColors: false,
		})

		reporter.Error(assert.AnError)

		output := buf.String()
		assert.Contains(t, output, "failed")
		assert.Contains(t, output, assert.AnError.Error())
	})

	t.Run("quiet mode suppresses error message", func(t *testing.T) {
		buf := &bytes.Buffer{}
		reporter := NewProgressReporter(ProgressReporterOptions{
			Writer: buf,
			Quiet:  true,
		})

		reporter.Error(assert.AnError)

		output := buf.String()
		assert.Empty(t, output)
	})
}

func TestProgressReporter_Callback(t *testing.T) {
	t.Run("callback function works correctly", func(t *testing.T) {
		buf := &bytes.Buffer{}
		reporter := NewProgressReporter(ProgressReporterOptions{
			Writer:    buf,
			UseColors: false,
		})

		callback := reporter.Callback()
		require.NotNil(t, callback)

		callback("test-stage", 50, "Test message")

		output := buf.String()
		assert.Contains(t, output, "Test Stage")
		assert.Contains(t, output, "50%")
		assert.Contains(t, output, "Test message")
	})
}

func TestProgressReporter_LegacyStageNameOutput(t *testing.T) {
	reporter := NewProgressReporter(ProgressReporterOptions{
		Writer:    &bytes.Buffer{},
		UseColors: false,
	})

	if got, want := reporter.formatStageName("go's-stage"), "Go'S Stage"; got != want {
		t.Errorf("formatStageName() = %q, want %q", got, want)
	}
}

func TestProgressReporter_ProgressBar(t *testing.T) {
	t.Run("progress bar shows correct fill level", func(t *testing.T) {
		buf := &bytes.Buffer{}
		reporter := NewProgressReporter(ProgressReporterOptions{
			Writer:    buf,
			UseColors: false,
		})

		reporter.Report("stage", 0, "0%")
		reporter.Report("stage", 50, "50%")
		reporter.Report("stage", 100, "100%")

		output := buf.String()

		// Count filled and empty characters in progress bars
		filledCount := strings.Count(output, "█")
		emptyCount := strings.Count(output, "░")

		// Should have some filled and empty characters
		assert.Greater(t, filledCount, 0)
		assert.Greater(t, emptyCount, 0)
	})
}

func TestProgressReporter_FormatDuration(t *testing.T) {
	reporter := NewProgressReporter(ProgressReporterOptions{
		Writer: &bytes.Buffer{},
	})

	tests := []struct {
		name     string
		duration time.Duration
		contains string
	}{
		{
			name:     "milliseconds",
			duration: 500 * time.Millisecond,
			contains: "ms",
		},
		{
			name:     "seconds",
			duration: 5 * time.Second,
			contains: "s",
		},
		{
			name:     "minutes",
			duration: 2 * time.Minute,
			contains: "m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatted := reporter.formatDuration(tt.duration)
			assert.Contains(t, formatted, tt.contains)
		})
	}
}

func TestProgressReporter_StageIcons(t *testing.T) {
	reporter := NewProgressReporter(ProgressReporterOptions{
		Writer:    &bytes.Buffer{},
		UseColors: false,
	})

	tests := []struct {
		stage        string
		expectedIcon string
	}{
		{"initialization", "🚀"},
		{"completion", "✓"},
		{"validation", "✓"},
		{"base", "📁"},
		{"infrastructure", "🏗️"},
		{"services", "⚙️"},
		{"config", "⚙️"},
		{"unknown", "▶"},
	}

	for _, tt := range tests {
		t.Run(tt.stage, func(t *testing.T) {
			icon := reporter.formatStageIcon(tt.stage)
			assert.Equal(t, tt.expectedIcon, icon)
		})
	}
}

func TestSimpleProgressReporter(t *testing.T) {
	t.Run("reports stage start and completion", func(t *testing.T) {
		buf := &bytes.Buffer{}
		reporter := NewSimpleProgressReporter(buf)

		callback := reporter.Callback()
		callback("test-stage", 0, "Starting")
		callback("test-stage", 50, "In progress")
		callback("test-stage", 100, "Done")

		output := buf.String()
		assert.Contains(t, output, "Starting: test-stage")
		assert.Contains(t, output, "Completed: test-stage")
		// Should not contain the 50% progress update
		assert.Equal(t, 2, strings.Count(output, "test-stage"))
	})
}

func TestProgressReporter_ThreadSafety(t *testing.T) {
	t.Run("concurrent reports are thread-safe", func(t *testing.T) {
		buf := &bytes.Buffer{}
		reporter := NewProgressReporter(ProgressReporterOptions{
			Writer:    buf,
			UseColors: false,
		})

		// Run multiple goroutines reporting progress
		done := make(chan bool)
		for i := 0; i < 10; i++ {
			go func(id int) {
				for j := 0; j <= 100; j += 10 {
					reporter.Report("concurrent-stage", j, "Processing")
				}
				done <- true
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}

		// Should not panic and should have output
		output := buf.String()
		assert.NotEmpty(t, output)
	})
}

func TestProgressReporter_Integration(t *testing.T) {
	t.Run("simulates full generation workflow", func(t *testing.T) {
		buf := &bytes.Buffer{}
		reporter := NewProgressReporter(ProgressReporterOptions{
			Writer:    buf,
			UseColors: false,
			Verbose:   true,
		})

		// Simulate initialization
		reporter.Report("initialization", 0, "Starting GitOps generation")
		reporter.Report("initialization", 100, "Initialization complete")

		// Simulate base structure stage
		reporter.Report("base-structure", 0, "Creating directory structure")
		reporter.Report("base-structure", 50, "Copying base files")
		reporter.Report("base-structure", 100, "Base structure complete")

		// Simulate infrastructure stage
		reporter.Report("infrastructure", 0, "Generating infrastructure templates")
		reporter.Report("infrastructure", 100, "Infrastructure complete")

		// Simulate completion
		reporter.Report("completion", 100, "Generation completed")
		reporter.Complete(5*time.Second, 42)

		output := buf.String()

		// Verify all stages are present
		assert.Contains(t, output, "Initialization")
		assert.Contains(t, output, "Base Structure")
		assert.Contains(t, output, "Infrastructure")
		assert.Contains(t, output, "Completion")

		// Verify progress percentages
		assert.Contains(t, output, "0%")
		assert.Contains(t, output, "50%")
		assert.Contains(t, output, "100%")

		// Verify completion message
		assert.Contains(t, output, "completed successfully")
		assert.Contains(t, output, "Files generated: 42")
	})
}

func TestNewDefaultProgressReporter(t *testing.T) {
	t.Run("creates reporter with default settings", func(t *testing.T) {
		buf := &bytes.Buffer{}
		reporter := NewDefaultProgressReporter(buf)

		require.NotNil(t, reporter)
		assert.False(t, reporter.quiet)
		assert.False(t, reporter.verbose)
		assert.True(t, reporter.useColors)
	})
}
