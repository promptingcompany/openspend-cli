package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type StepState string

const (
	StepPending StepState = "pending"
	StepRunning StepState = "running"
	StepDone    StepState = "done"
	StepError   StepState = "error"
)

type Step struct {
	Name    string
	State   StepState
	Message string
}

type Model struct {
	Title string
	Steps []Step
	Done  bool
}

type stepUpdateMsg struct {
	Index   int
	State   StepState
	Message string
}

type doneMsg struct{}

func NewOnboardingModel() Model {
	return Model{
		Title: "OpenSpend Buyer Quickstart",
		Steps: []Step{
			{Name: "Authenticate"},
			{Name: "Initialize buyer policy"},
			{Name: "Create buyer agent"},
			{Name: "Verify context"},
		},
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.KeyMsg:
		if v.String() == "q" || v.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case stepUpdateMsg:
		if v.Index >= 0 && v.Index < len(m.Steps) {
			m.Steps[v.Index].State = v.State
			m.Steps[v.Index].Message = v.Message
		}
	case doneMsg:
		m.Done = true
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) View() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Render(m.Title)
	var b strings.Builder
	b.WriteString(title + "\n\n")

	for _, step := range m.Steps {
		icon := "·"
		switch step.State {
		case StepRunning:
			icon = "…"
		case StepDone:
			icon = "✓"
		case StepError:
			icon = "x"
		}

		line := fmt.Sprintf("%s %s", icon, step.Name)
		if step.Message != "" {
			line += fmt.Sprintf(" - %s", step.Message)
		}
		b.WriteString(line + "\n")
	}

	if !m.Done {
		b.WriteString("\nPress q to quit.\n")
	}
	return b.String()
}

func StepUpdate(index int, state StepState, message string) tea.Msg {
	return stepUpdateMsg{Index: index, State: state, Message: message}
}

func Done() tea.Msg {
	return doneMsg{}
}
