package ui

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencenter-cloud/opencenter-cli/internal/cluster/orchestration"
	"golang.org/x/term"
)

const guidedAnswersEnv = "OPENCENTER_GUIDED_ANSWERS"

var (
	guidedDocStyle      = lipgloss.NewStyle().Margin(1, 2)
	guidedTitleStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFDF5")).Background(lipgloss.Color("#25A065")).Padding(0, 1)
	guidedSelectedStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
)

type guidedTerminalInput interface {
	io.Reader
	Fd() uintptr
}

type InteractiveGuidedPromptRunner struct {
	input       io.Reader
	output      io.Writer
	errorOutput io.Writer
	reader      *bufio.Reader
}

func NewInteractiveGuidedPromptRunner(input io.Reader, output io.Writer, errorOutput io.Writer) *InteractiveGuidedPromptRunner {
	if input == nil {
		input = os.Stdin
	}
	if output == nil {
		output = os.Stdout
	}
	if errorOutput == nil {
		errorOutput = output
	}

	return &InteractiveGuidedPromptRunner{
		input:       input,
		output:      output,
		errorOutput: errorOutput,
		reader:      bufio.NewReader(input),
	}
}

func GetGuidedPromptRunner(input io.Reader, output io.Writer, errorOutput io.Writer, testMode bool) orchestration.PromptRunner {
	if testMode {
		if raw := strings.TrimSpace(os.Getenv(guidedAnswersEnv)); raw != "" {
			runner, err := NewScriptedGuidedPromptRunnerFromJSON(raw, output)
			if err == nil {
				return runner
			}
		}
	}
	return NewInteractiveGuidedPromptRunner(input, output, errorOutput)
}

func (p *InteractiveGuidedPromptRunner) Message(message string) {
	if strings.TrimSpace(message) == "" {
		return
	}
	_, _ = fmt.Fprintln(p.output, message)
}

func (p *InteractiveGuidedPromptRunner) Warning(message string) {
	if strings.TrimSpace(message) == "" {
		return
	}
	_, _ = fmt.Fprintf(p.errorOutput, "warning: %s\n", message)
}

func (p *InteractiveGuidedPromptRunner) Prompt(ctx context.Context, prompts []orchestration.PromptSpec) (orchestration.PromptAnswers, error) {
	answers := make(orchestration.PromptAnswers, len(prompts))

	for _, prompt := range prompts {
		value, err := p.promptSingle(ctx, prompt)
		if err != nil {
			return nil, err
		}
		if err := validatePromptValue(prompt, value); err != nil {
			return nil, err
		}
		answers[prompt.ID] = value
	}

	return answers, nil
}

func (p *InteractiveGuidedPromptRunner) Review(ctx context.Context, review orchestration.ReviewSpec) (bool, error) {
	if strings.TrimSpace(review.Title) != "" {
		_, _ = fmt.Fprintf(p.output, "%s\n", review.Title)
	}
	for _, group := range review.Groups {
		if strings.TrimSpace(group.Name) != "" {
			_, _ = fmt.Fprintf(p.output, "\n[%s]\n", group.Name)
		}
		for _, entry := range group.Entries {
			value := entry.Value
			if entry.Masked {
				value = "********"
			}
			_, _ = fmt.Fprintf(p.output, "  %s: %s\n", entry.Label, value)
		}
	}

	prompter := NewInteractivePrompter(p.input, p.output)
	return prompter.Confirm(ctx, "Apply these guided configuration changes?")
}

func (p *InteractiveGuidedPromptRunner) promptSingle(ctx context.Context, prompt orchestration.PromptSpec) (string, error) {
	switch prompt.Kind {
	case orchestration.PromptKindInput:
		return p.promptInput(ctx, prompt)
	case orchestration.PromptKindSecret:
		return p.promptSecret(ctx, prompt)
	case orchestration.PromptKindSelect:
		return p.promptSelect(ctx, prompt)
	case orchestration.PromptKindConfirm:
		return p.promptConfirm(ctx, prompt)
	default:
		return "", fmt.Errorf("unsupported guided prompt kind %q", prompt.Kind)
	}
}

func (p *InteractiveGuidedPromptRunner) promptInput(ctx context.Context, prompt orchestration.PromptSpec) (string, error) {
	_, _ = fmt.Fprint(p.output, formatPromptLabel(prompt))
	value, err := p.readLine(ctx)
	if err != nil {
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return prompt.Default, nil
	}
	return value, nil
}

func (p *InteractiveGuidedPromptRunner) promptSecret(ctx context.Context, prompt orchestration.PromptSpec) (string, error) {
	if stdinFile, ok := p.input.(guidedTerminalInput); ok && term.IsTerminal(int(stdinFile.Fd())) {
		if _, err := fmt.Fprint(p.output, formatPromptLabel(prompt)); err != nil {
			return "", err
		}
		valueBytes, err := term.ReadPassword(int(stdinFile.Fd()))
		if _, newlineErr := fmt.Fprintln(p.output); newlineErr != nil && err == nil {
			err = newlineErr
		}
		if err != nil {
			return "", err
		}
		value := strings.TrimSpace(string(valueBytes))
		if value == "" {
			return prompt.Default, nil
		}
		return value, nil
	}

	return p.promptInput(ctx, prompt)
}

func (p *InteractiveGuidedPromptRunner) promptSelect(ctx context.Context, prompt orchestration.PromptSpec) (string, error) {
	if len(prompt.Options) == 0 {
		return "", fmt.Errorf("select prompt %q has no options", prompt.ID)
	}

	if stdinFile, ok := p.input.(guidedTerminalInput); ok && term.IsTerminal(int(stdinFile.Fd())) {
		items := make([]list.Item, 0, len(prompt.Options))
		defaultIndex := 0
		for idx, option := range prompt.Options {
			items = append(items, guidedSelectItem{
				value:       option.Value,
				title:       option.Label,
				description: option.Description,
			})
			if prompt.Default != "" && option.Value == prompt.Default {
				defaultIndex = idx
			}
		}

		delegate := list.NewDefaultDelegate()
		delegate.Styles.SelectedTitle = guidedSelectedStyle
		delegate.Styles.SelectedDesc = guidedSelectedStyle

		component := list.New(items, delegate, 0, 0)
		component.Title = prompt.Label
		component.SetShowStatusBar(false)
		component.SetFilteringEnabled(len(items) > 8)
		component.Select(defaultIndex)

		model := guidedSelectModel{list: component}
		program := tea.NewProgram(model, tea.WithInput(p.input), tea.WithOutput(p.output))
		result, err := program.Run()
		if err != nil {
			return "", err
		}
		finalModel, ok := result.(guidedSelectModel)
		if !ok {
			return "", fmt.Errorf("unexpected Bubble Tea result type %T", result)
		}
		if finalModel.choice == "" {
			return prompt.Default, nil
		}
		return finalModel.choice, nil
	}

	_, _ = fmt.Fprintf(p.output, "%s\n", prompt.Label)
	for idx, option := range prompt.Options {
		_, _ = fmt.Fprintf(p.output, "  %d. %s\n", idx+1, option.Label)
	}
	_, _ = fmt.Fprint(p.output, "Select an option")
	if prompt.Default != "" {
		_, _ = fmt.Fprintf(p.output, " [%s]", prompt.Default)
	}
	_, _ = fmt.Fprint(p.output, ": ")

	value, err := p.readLine(ctx)
	if err != nil {
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return prompt.Default, nil
	}
	if index, err := strconv.Atoi(value); err == nil && index >= 1 && index <= len(prompt.Options) {
		return prompt.Options[index-1].Value, nil
	}
	for _, option := range prompt.Options {
		if option.Value == value {
			return value, nil
		}
	}
	return "", fmt.Errorf("invalid selection %q for %s", value, prompt.Label)
}

func (p *InteractiveGuidedPromptRunner) promptConfirm(ctx context.Context, prompt orchestration.PromptSpec) (string, error) {
	prompter := NewInteractivePrompter(p.input, p.output)
	confirmed, err := prompter.Confirm(ctx, prompt.Label)
	if err != nil {
		return "", err
	}
	if confirmed {
		return "true", nil
	}
	return "false", nil
}

func (p *InteractiveGuidedPromptRunner) readLine(ctx context.Context) (string, error) {
	type response struct {
		value string
		err   error
	}
	ch := make(chan response, 1)
	go func() {
		value, err := p.reader.ReadString('\n')
		ch <- response{value: value, err: err}
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case result := <-ch:
		if result.err != nil && result.err != io.EOF {
			return "", result.err
		}
		return strings.TrimRight(result.value, "\r\n"), nil
	}
}

type ScriptedGuidedPromptRunner struct {
	answers map[string]string
	output  io.Writer
}

func NewScriptedGuidedPromptRunnerFromJSON(raw string, output io.Writer) (*ScriptedGuidedPromptRunner, error) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, fmt.Errorf("parse %s: %w", guidedAnswersEnv, err)
	}

	answers := make(map[string]string, len(payload))
	for key, value := range payload {
		switch typed := value.(type) {
		case string:
			answers[key] = typed
		case bool:
			answers[key] = strconv.FormatBool(typed)
		case float64:
			answers[key] = strconv.Itoa(int(typed))
		default:
			answers[key] = fmt.Sprint(value)
		}
	}

	return &ScriptedGuidedPromptRunner{answers: answers, output: output}, nil
}

func (p *ScriptedGuidedPromptRunner) Message(message string) {
	if strings.TrimSpace(message) == "" || p.output == nil {
		return
	}
	_, _ = fmt.Fprintln(p.output, message)
}

func (p *ScriptedGuidedPromptRunner) Warning(message string) {
	if strings.TrimSpace(message) == "" || p.output == nil {
		return
	}
	_, _ = fmt.Fprintf(p.output, "warning: %s\n", message)
}

func (p *ScriptedGuidedPromptRunner) Prompt(_ context.Context, prompts []orchestration.PromptSpec) (orchestration.PromptAnswers, error) {
	answers := make(orchestration.PromptAnswers, len(prompts))
	for _, prompt := range prompts {
		value, ok := p.answers[prompt.ID]
		if !ok {
			value = prompt.Default
		}
		if err := validatePromptValue(prompt, value); err != nil {
			return nil, err
		}
		answers[prompt.ID] = value
	}
	return answers, nil
}

func (p *ScriptedGuidedPromptRunner) Review(_ context.Context, _ orchestration.ReviewSpec) (bool, error) {
	value, ok := p.answers["review.confirm"]
	if !ok || strings.TrimSpace(value) == "" {
		return true, nil
	}
	return strconv.ParseBool(strings.TrimSpace(value))
}

func validatePromptValue(prompt orchestration.PromptSpec, value string) error {
	if prompt.Required && strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", prompt.Label)
	}
	if prompt.Validate != nil {
		if err := prompt.Validate(value); err != nil {
			return fmt.Errorf("%s: %w", prompt.Label, err)
		}
	}
	return nil
}

func formatPromptLabel(prompt orchestration.PromptSpec) string {
	label := prompt.Label
	if prompt.Default != "" {
		label = fmt.Sprintf("%s [%s]", label, prompt.Default)
	}
	return label + ": "
}

type guidedSelectItem struct {
	value       string
	title       string
	description string
}

func (i guidedSelectItem) Title() string       { return i.title }
func (i guidedSelectItem) Description() string { return i.description }
func (i guidedSelectItem) FilterValue() string { return i.title }

type guidedSelectModel struct {
	list     list.Model
	choice   string
	quitting bool
}

func (m guidedSelectModel) Init() tea.Cmd {
	return nil
}

func (m guidedSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			item, ok := m.list.SelectedItem().(guidedSelectItem)
			if ok {
				m.choice = item.value
			}
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		h, v := guidedDocStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m guidedSelectModel) View() string {
	if m.choice != "" || m.quitting {
		return ""
	}
	return guidedDocStyle.Render(guidedTitleStyle.Render(m.list.Title) + "\n" + m.list.View())
}
