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
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// ProgressReporter provides user-facing progress reporting for GitOps generation.
// It displays real-time progress updates with stage information and completion status.
type ProgressReporter struct {
	writer       io.Writer
	mu           sync.Mutex
	currentStage string
	lastProgress int
	startTime    time.Time
	quiet        bool
	verbose      bool
	useColors    bool
}

// ProgressReporterOptions configures the progress reporter.
type ProgressReporterOptions struct {
	// Writer is where progress output is written (typically os.Stdout)
	Writer io.Writer

	// Quiet suppresses all progress output
	Quiet bool

	// Verbose enables detailed progress messages
	Verbose bool

	// UseColors enables colored output (ANSI escape codes)
	UseColors bool
}

// NewProgressReporter creates a new progress reporter with the given options.
func NewProgressReporter(opts ProgressReporterOptions) *ProgressReporter {
	if opts.Writer == nil {
		opts.Writer = io.Discard
	}

	return &ProgressReporter{
		writer:    opts.Writer,
		quiet:     opts.Quiet,
		verbose:   opts.Verbose,
		useColors: opts.UseColors,
		startTime: time.Now(),
	}
}

// NewDefaultProgressReporter creates a progress reporter with default settings.
func NewDefaultProgressReporter(writer io.Writer) *ProgressReporter {
	return NewProgressReporter(ProgressReporterOptions{
		Writer:    writer,
		Quiet:     false,
		Verbose:   false,
		UseColors: true,
	})
}

// Callback returns a ProgressCallback function that can be used with GitOpsGenerator.
func (pr *ProgressReporter) Callback() ProgressCallback {
	return func(stage string, progress int, message string) {
		pr.Report(stage, progress, message)
	}
}

// Report displays a progress update to the user.
func (pr *ProgressReporter) Report(stage string, progress int, message string) {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	if pr.quiet {
		return
	}

	// Update current stage
	if stage != pr.currentStage {
		pr.currentStage = stage
		pr.lastProgress = 0

		// Print stage header
		if pr.verbose {
			fmt.Fprintf(pr.writer, "\n")
		}
		fmt.Fprintf(pr.writer, "%s %s\n", pr.formatStageIcon(stage), pr.formatStageName(stage))
	}

	// Update progress if it changed
	if progress != pr.lastProgress {
		pr.lastProgress = progress
	}

	// Print progress bar and message
	if pr.verbose {
		elapsed := time.Since(pr.startTime)
		fmt.Fprintf(pr.writer, "  [%s] %3d%% - %s (elapsed: %s)\n",
			pr.formatProgressBar(progress),
			progress,
			message,
			pr.formatDuration(elapsed))
	} else {
		fmt.Fprintf(pr.writer, "  [%s] %3d%% - %s\n",
			pr.formatProgressBar(progress),
			progress,
			message)
	}
}

// Complete displays a completion message.
func (pr *ProgressReporter) Complete(duration time.Duration, filesGenerated int) {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	if pr.quiet {
		return
	}

	fmt.Fprintf(pr.writer, "\n%s Generation completed successfully!\n", pr.colorize("✓", colorGreen))
	fmt.Fprintf(pr.writer, "  Duration: %s\n", pr.formatDuration(duration))
	fmt.Fprintf(pr.writer, "  Files generated: %d\n", filesGenerated)
}

// Error displays an error message.
func (pr *ProgressReporter) Error(err error) {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	if pr.quiet {
		return
	}

	fmt.Fprintf(pr.writer, "\n%s Generation failed: %v\n", pr.colorize("✗", colorRed), err)
}

// formatProgressBar creates a visual progress bar.
func (pr *ProgressReporter) formatProgressBar(progress int) string {
	const barWidth = 20
	filled := int(float64(progress) / 100.0 * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	if pr.useColors {
		if progress == 100 {
			return pr.colorize(bar, colorGreen)
		} else if progress >= 50 {
			return pr.colorize(bar, colorYellow)
		}
		return pr.colorize(bar, colorCyan)
	}

	return bar
}

// formatStageName formats a stage name for display.
func (pr *ProgressReporter) formatStageName(stage string) string {
	// Convert stage names to human-readable format
	name := strings.ReplaceAll(stage, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")
	//lint:ignore SA1019 strings.Title preserves legacy stage labels for compatibility.
	//nolint:staticcheck // SA1019: preserve legacy stage label formatting behavior.
	name = strings.Title(name)

	if pr.useColors {
		return pr.colorize(name, colorBold)
	}

	return name
}

// formatStageIcon returns an icon for the stage.
func (pr *ProgressReporter) formatStageIcon(stage string) string {
	icon := "▶"

	switch stage {
	case "initialization":
		icon = "🚀"
	case "completion":
		icon = "✓"
	case "validation":
		icon = "✓"
	case "base", "base-structure":
		icon = "📁"
	case "infrastructure":
		icon = "🏗️"
	case "services":
		icon = "⚙️"
	case "config", "configuration":
		icon = "⚙️"
	}

	if pr.useColors {
		return pr.colorize(icon, colorCyan)
	}

	return icon
}

// formatDuration formats a duration for display.
func (pr *ProgressReporter) formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	} else if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%.1fm", d.Minutes())
}

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
)

// colorize applies ANSI color codes to text if colors are enabled.
func (pr *ProgressReporter) colorize(text, color string) string {
	if !pr.useColors {
		return text
	}
	return color + text + colorReset
}

// SimpleProgressReporter provides a minimal progress reporter for non-interactive use.
type SimpleProgressReporter struct {
	writer io.Writer
	mu     sync.Mutex
}

// NewSimpleProgressReporter creates a simple progress reporter that just prints stage names.
func NewSimpleProgressReporter(writer io.Writer) *SimpleProgressReporter {
	return &SimpleProgressReporter{
		writer: writer,
	}
}

// Callback returns a ProgressCallback function.
func (spr *SimpleProgressReporter) Callback() ProgressCallback {
	return func(stage string, progress int, message string) {
		spr.mu.Lock()
		defer spr.mu.Unlock()

		switch progress {
		case 0:
			fmt.Fprintf(spr.writer, "Starting: %s\n", stage)
		case 100:
			fmt.Fprintf(spr.writer, "Completed: %s\n", stage)
		}
	}
}
