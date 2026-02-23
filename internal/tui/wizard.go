package tui

import (
	"fmt"
	"strconv"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pdiegmann/imap-eml-export/internal/config"
	"github.com/pdiegmann/imap-eml-export/internal/google"
)

// wizardMode distinguishes between the export and import setup wizards.
type wizardMode int

const (
	wizardExport wizardMode = iota
	wizardImport
)

type wizardStep int

const (
	stepProvider  wizardStep = iota // provider selection: IMAP or Google
	stepHost                        // IMAP only
	stepPort                        // IMAP only
	stepUsername                    // always required
	stepPassword                    // IMAP only
	stepDirectory                   // output_dir (export) or input_dir (import)
	stepDone
)

// Input slice indices – independent of step enum values.
const (
	idxHost      = 0
	idxPort      = 1
	idxUsername  = 2
	idxPassword  = 3
	idxDirectory = 4
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))
	promptStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	hintStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true)
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	successStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))
)

type wizardModel struct {
	mode         wizardMode
	step         wizardStep
	inputs       []textinput.Model
	providerIdx  int  // 0 = IMAP, 1 = Google
	useGoogle    bool // resolved after provider selection
	skipProvider bool // true when provider was already set via a CLI flag
	err          string
	completed    bool
	cfg          *config.Config
}

// stepInputIdx returns the index into m.inputs for a given step, or -1 if none.
func stepInputIdx(s wizardStep) int {
	switch s {
	case stepHost:
		return idxHost
	case stepPort:
		return idxPort
	case stepUsername:
		return idxUsername
	case stepPassword:
		return idxPassword
	case stepDirectory:
		return idxDirectory
	}
	return -1
}

func newWizardModel(mode wizardMode, presetGoogle bool) wizardModel {
	inputs := make([]textinput.Model, 5)

	inputs[idxHost] = textinput.New()
	inputs[idxHost].Placeholder = "imap.example.com"
	inputs[idxHost].CharLimit = 256

	inputs[idxPort] = textinput.New()
	inputs[idxPort].Placeholder = "993"
	inputs[idxPort].CharLimit = 5

	inputs[idxUsername] = textinput.New()
	inputs[idxUsername].Placeholder = "user@example.com"
	inputs[idxUsername].CharLimit = 256

	inputs[idxPassword] = textinput.New()
	inputs[idxPassword].Placeholder = "••••••••"
	inputs[idxPassword].EchoMode = textinput.EchoPassword
	inputs[idxPassword].EchoCharacter = '•'
	inputs[idxPassword].CharLimit = 256

	inputs[idxDirectory] = textinput.New()
	if mode == wizardExport {
		inputs[idxDirectory].Placeholder = "./output"
	} else {
		inputs[idxDirectory].Placeholder = "./input"
	}
	inputs[idxDirectory].CharLimit = 512

	m := wizardModel{
		mode:         mode,
		inputs:       inputs,
		skipProvider: presetGoogle,
		useGoogle:    presetGoogle,
	}

	if presetGoogle {
		// Provider is already known; start directly at the username step.
		m.step = stepUsername
		m.providerIdx = 1
		inputs[idxUsername].Focus()
	} else {
		m.step = stepProvider
	}

	return m
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
		case tea.KeyUp:
			if m.step == stepProvider && m.providerIdx > 0 {
				m.providerIdx--
			}
			return m, nil
		case tea.KeyDown:
			if m.step == stepProvider && m.providerIdx < 1 {
				m.providerIdx++
			}
			return m, nil
		}
		// Digit shortcuts for provider selection.
		if m.step == stepProvider {
			switch msg.String() {
			case "1":
				m.providerIdx = 0
				return m, nil
			case "2":
				m.providerIdx = 1
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	if idx := stepInputIdx(m.step); idx >= 0 {
		m.inputs[idx], cmd = m.inputs[idx].Update(msg)
	}
	return m, cmd
}

func (m wizardModel) nextStep() (tea.Model, tea.Cmd) {
	m.err = ""

	switch m.step {
	case stepProvider:
		m.useGoogle = m.providerIdx == 1
		if m.useGoogle {
			m.step = stepUsername
		} else {
			m.step = stepHost
		}
		m.inputs[stepInputIdx(m.step)].Focus()
		return m, textinput.Blink

	case stepHost:
		if m.inputs[idxHost].Value() == "" {
			m.err = "host is required"
			return m, nil
		}
		m.step = stepPort
		m.inputs[idxPort].Focus()
		return m, textinput.Blink

	case stepPort:
		val := m.inputs[idxPort].Value()
		if val == "" {
			val = "993"
			m.inputs[idxPort].SetValue(val)
		}
		if _, err := strconv.Atoi(val); err != nil {
			m.err = "port must be a number"
			return m, nil
		}
		m.step = stepUsername
		m.inputs[idxUsername].Focus()
		return m, textinput.Blink

	case stepUsername:
		if m.inputs[idxUsername].Value() == "" {
			m.err = "username is required"
			return m, nil
		}
		if m.useGoogle {
			m.step = stepDirectory
		} else {
			m.step = stepPassword
		}
		m.inputs[stepInputIdx(m.step)].Focus()
		return m, textinput.Blink

	case stepPassword:
		if m.inputs[idxPassword].Value() == "" {
			m.err = "password is required"
			return m, nil
		}
		m.step = stepDirectory
		m.inputs[idxDirectory].Focus()
		return m, textinput.Blink

	case stepDirectory:
		val := m.inputs[idxDirectory].Value()
		if val == "" {
			if m.mode == wizardExport {
				val = "./output"
			} else {
				val = "./input"
			}
			m.inputs[idxDirectory].SetValue(val)
		}
		m.completed = true
		m.buildConfig()
		return m, tea.Quit
	}

	return m, nil
}

func (m *wizardModel) buildConfig() {
	port, _ := strconv.Atoi(m.inputs[idxPort].Value())
	if port == 0 {
		port = 993
	}

	if m.mode == wizardExport {
		exp := config.ExportConfig{
			Host:      m.inputs[idxHost].Value(),
			Port:      port,
			Username:  m.inputs[idxUsername].Value(),
			Password:  m.inputs[idxPassword].Value(),
			OutputDir: m.inputs[idxDirectory].Value(),
			TLS:       port == 993,
			Google:    m.useGoogle,
		}
		if m.useGoogle {
			exp.Host = google.GmailIMAPHost
			exp.Port = google.GmailIMAPPort
			exp.TLS = true
		}
		m.cfg = &config.Config{Export: exp}
	} else {
		imp := config.ImportConfig{
			Host:     m.inputs[idxHost].Value(),
			Port:     port,
			Username: m.inputs[idxUsername].Value(),
			Password: m.inputs[idxPassword].Value(),
			InputDir: m.inputs[idxDirectory].Value(),
			TLS:      port == 993,
			Google:   m.useGoogle,
		}
		if m.useGoogle {
			imp.Host = google.GmailIMAPHost
			imp.Port = google.GmailIMAPPort
			imp.TLS = true
		}
		m.cfg = &config.Config{Import: imp}
	}
}

func (m wizardModel) View() string {
	if m.completed {
		return successStyle.Render("✓ Configuration complete!") + "\n"
	}

	dirLabel := "Output Directory"
	dirHint := "The directory where exported .eml files will be written.\n  It will be created if it does not exist."
	if m.mode == wizardImport {
		dirLabel = "Input Directory"
		dirHint = "The directory containing .eml files to import."
	}

	view := titleStyle.Render("IMAP EML Export - Setup Wizard") + "\n"
	view += hintStyle.Render("Answer each question and press Enter. Leave blank to accept the default.") + "\n\n"

	if m.step == stepProvider {
		view += promptStyle.Render("▸ Connection Type:") + "\n"
		providers := []string{"Standard IMAP", "Google / Gmail (OAuth2)"}
		for i, p := range providers {
			if i == m.providerIdx {
				view += selectedStyle.Render(fmt.Sprintf("  ❯ [%d] %s", i+1, p)) + "\n"
			} else {
				view += promptStyle.Render(fmt.Sprintf("    [%d] %s", i+1, p)) + "\n"
			}
		}
		view += hintStyle.Render("  Use ↑/↓ or 1/2 to select, then press Enter") + "\n"
	} else {
		// Show the resolved provider as a completed item.
		providerName := "Standard IMAP"
		if m.useGoogle {
			providerName = "Google / Gmail (OAuth2)"
		}
		view += promptStyle.Render("  Connection Type: ") + providerName + "\n"

		// Ordered list of fields to display.
		type fieldDef struct {
			step  wizardStep
			label string
			hint  string
		}
		fields := []fieldDef{
			{stepHost, "IMAP Host", "Common values: imap.gmail.com · imap.mail.yahoo.com · outlook.office365.com"},
			{stepPort, "Port", "993 = implicit TLS/IMAPS (recommended)  |  143 = plain / STARTTLS"},
			{stepUsername, "Username", "Usually your full email address, e.g. you@example.com"},
			{stepPassword, "Password", "Gmail / Google Workspace: create an App Password at\n    myaccount.google.com/apppasswords  (requires 2-Step Verification)"},
			{stepDirectory, dirLabel, dirHint},
		}

		for _, f := range fields {
			// Skip fields that don't apply to Google mode.
			if m.useGoogle && (f.step == stepHost || f.step == stepPort || f.step == stepPassword) {
				continue
			}
			idx := stepInputIdx(f.step)
			if f.step == m.step {
				view += promptStyle.Render(fmt.Sprintf("▸ %s: ", f.label))
				view += m.inputs[idx].View() + "\n"
				view += hintStyle.Render("  "+f.hint) + "\n"
			} else if f.step < m.step {
				view += promptStyle.Render(fmt.Sprintf("  %s: ", f.label))
				if f.step == stepPassword {
					view += "••••••••\n"
				} else {
					view += m.inputs[idx].Value() + "\n"
				}
			}
		}
	}

	if m.err != "" {
		view += "\n" + errorStyle.Render("✗ "+m.err) + "\n"
	}
	view += "\n" + promptStyle.Render("Enter = next  |  Esc / Ctrl+C = quit")
	return view
}

// RunWizard launches the interactive export setup wizard.
// Deprecated: Use RunExportWizard instead.
func RunWizard() (*config.Config, error) {
	return RunExportWizard(false)
}

// RunExportWizard launches the interactive export setup wizard.
// If presetGoogle is true, the provider selection step is skipped and the
// wizard starts directly at the username prompt (for use when --google was
// already supplied on the command line but the username is still missing).
func RunExportWizard(presetGoogle bool) (*config.Config, error) {
	return runWizard(wizardExport, presetGoogle)
}

// RunImportWizard launches the interactive import setup wizard.
// If presetGoogle is true, the provider selection step is skipped and the
// wizard starts directly at the username prompt.
func RunImportWizard(presetGoogle bool) (*config.Config, error) {
	return runWizard(wizardImport, presetGoogle)
}

func runWizard(mode wizardMode, presetGoogle bool) (*config.Config, error) {
	m := newWizardModel(mode, presetGoogle)
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
