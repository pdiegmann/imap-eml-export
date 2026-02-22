package tui

import (
	"fmt"
	"strconv"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pdiegmann/imap-eml-export/internal/config"
)

type wizardStep int

const (
	stepHost wizardStep = iota
	stepPort
	stepUsername
	stepPassword
	stepOutputDir
	stepDone
)

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))
	promptStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
)

type wizardModel struct {
	step      wizardStep
	inputs    []textinput.Model
	err       string
	completed bool
	cfg       *config.Config
}

func newWizardModel() wizardModel {
	inputs := make([]textinput.Model, 5)

	inputs[0] = textinput.New()
	inputs[0].Placeholder = "imap.gmail.com"
	inputs[0].Focus()
	inputs[0].CharLimit = 256

	inputs[1] = textinput.New()
	inputs[1].Placeholder = "993"
	inputs[1].CharLimit = 5

	inputs[2] = textinput.New()
	inputs[2].Placeholder = "user@example.com"
	inputs[2].CharLimit = 256

	inputs[3] = textinput.New()
	inputs[3].Placeholder = "••••••••"
	inputs[3].EchoMode = textinput.EchoPassword
	inputs[3].EchoCharacter = '•'
	inputs[3].CharLimit = 256

	inputs[4] = textinput.New()
	inputs[4].Placeholder = "./output"
	inputs[4].CharLimit = 512

	return wizardModel{
		step:   stepHost,
		inputs: inputs,
	}
}

func (m wizardModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m wizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			return m.nextStep()
		}
	}

	var cmd tea.Cmd
	if int(m.step) < len(m.inputs) {
		m.inputs[m.step], cmd = m.inputs[m.step].Update(msg)
	}
	return m, cmd
}

func (m wizardModel) nextStep() (tea.Model, tea.Cmd) {
	val := m.inputs[m.step].Value()
	m.err = ""

	switch m.step {
	case stepHost:
		if val == "" {
			m.err = "host is required"
			return m, nil
		}
	case stepPort:
		if val == "" {
			val = "993"
			m.inputs[stepPort].SetValue(val)
		}
		if _, err := strconv.Atoi(val); err != nil {
			m.err = "port must be a number"
			return m, nil
		}
	case stepUsername:
		if val == "" {
			m.err = "username is required"
			return m, nil
		}
	case stepPassword:
		if val == "" {
			m.err = "password is required"
			return m, nil
		}
	case stepOutputDir:
		if val == "" {
			val = "./output"
			m.inputs[stepOutputDir].SetValue(val)
		}
		m.completed = true
		m.buildConfig()
		return m, tea.Quit
	}

	m.step++
	m.inputs[m.step].Focus()
	return m, textinput.Blink
}

func (m *wizardModel) buildConfig() {
	port, _ := strconv.Atoi(m.inputs[stepPort].Value())
	if port == 0 {
		port = 993
	}
	m.cfg = &config.Config{
		Host:      m.inputs[stepHost].Value(),
		Port:      port,
		Username:  m.inputs[stepUsername].Value(),
		Password:  m.inputs[stepPassword].Value(),
		OutputDir: m.inputs[stepOutputDir].Value(),
		TLS:       port == 993,
	}
}

func (m wizardModel) View() string {
	labels := []string{"IMAP Host", "Port", "Username", "Password", "Output Directory"}
	if m.completed {
		return successStyle.Render("✓ Configuration complete!") + "\n"
	}

	view := titleStyle.Render("IMAP EML Export - Setup Wizard") + "\n\n"
	for i, label := range labels {
		if i == int(m.step) {
			view += promptStyle.Render(fmt.Sprintf("▸ %s: ", label))
			view += m.inputs[i].View() + "\n"
		} else if i < int(m.step) {
			view += promptStyle.Render(fmt.Sprintf("  %s: ", label))
			if i == int(stepPassword) {
				view += "••••••••\n"
			} else {
				view += m.inputs[i].Value() + "\n"
			}
		}
	}
	if m.err != "" {
		view += "\n" + errorStyle.Render("Error: "+m.err) + "\n"
	}
	view += "\n" + promptStyle.Render("Press Enter to continue, Esc to quit")
	return view
}

// RunWizard launches the interactive setup wizard and returns the resulting config.
func RunWizard() (*config.Config, error) {
	m := newWizardModel()
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("wizard error: %w", err)
	}
	wm, ok := finalModel.(wizardModel)
	if !ok || !wm.completed {
		return nil, fmt.Errorf("wizard cancelled")
	}
	return wm.cfg, nil
}
