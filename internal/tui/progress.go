package tui

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pdiegmann/imap-eml-export/internal/export"
)

type progressModel struct {
	spinner   spinner.Model
	progress  progress.Model
	update    export.ProgressUpdate
	startTime time.Time
	done      bool
	verb      string // e.g. "Exporting" or "Importing"
	noun      string // e.g. "Export" or "Import"
}

type progressUpdateMsg export.ProgressUpdate
type doneMsg struct{}

func newProgressModel() progressModel {
	return newProgressModelLabeled("Exporting", "Export")
}

func newProgressModelLabeled(verb, noun string) progressModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	p := progress.New(progress.WithDefaultGradient())

	return progressModel{
		spinner:   s,
		progress:  p,
		startTime: time.Now(),
		verb:      verb,
		noun:      noun,
	}
}

func (m progressModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m progressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
	case progressUpdateMsg:
		m.update = export.ProgressUpdate(msg)
		var pct float64
		if msg.Total > 0 {
			pct = float64(msg.Current) / float64(msg.Total)
		}
		return m, m.progress.SetPercent(pct)
	case doneMsg:
		m.done = true
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case progress.FrameMsg:
		pm, cmd := m.progress.Update(msg)
		m.progress = pm.(progress.Model)
		return m, cmd
	}
	return m, nil
}

func (m progressModel) View() string {
	if m.done {
		elapsed := time.Since(m.startTime).Round(time.Second)
		return successStyle.Render(fmt.Sprintf("✓ %s complete in %s!", m.noun, elapsed)) + "\n"
	}

	elapsed := time.Since(m.startTime).Round(time.Second)
	var msgsPerSec float64
	if elapsed.Seconds() > 0 {
		msgsPerSec = float64(m.update.Current) / elapsed.Seconds()
	}

	return fmt.Sprintf(
		"%s %s: %s\n%s\n%d/%d messages | %.1f msg/s | %s elapsed\n",
		m.spinner.View(),
		m.verb,
		m.update.Folder,
		m.progress.View(),
		m.update.Current,
		m.update.Total,
		msgsPerSec,
		elapsed,
	)
}

// RunProgress displays an export progress dashboard.
func RunProgress(ctx context.Context, updates <-chan export.ProgressUpdate) error {
	return runProgressLabeled(ctx, updates, "Exporting", "Export")
}

// RunImportProgress displays an import progress dashboard.
func RunImportProgress(ctx context.Context, updates <-chan export.ProgressUpdate) error {
	return runProgressLabeled(ctx, updates, "Importing", "Import")
}

func runProgressLabeled(ctx context.Context, updates <-chan export.ProgressUpdate, verb, noun string) error {
	m := newProgressModelLabeled(verb, noun)
	p := tea.NewProgram(m)

	go func() {
		for {
			select {
			case <-ctx.Done():
				p.Send(doneMsg{})
				return
			case upd, ok := <-updates:
				if !ok {
					p.Send(doneMsg{})
					return
				}
				p.Send(progressUpdateMsg(upd))
			}
		}
	}()

	_, err := p.Run()
	return err
}
