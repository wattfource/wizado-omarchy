package tui

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// LogsModel holds logs for display
type LogsModel struct {
	lines      []string
	scroll     int
	maxLines   int
	logType    string // "main" or "session"
	loading    bool
}

// NewLogsModel creates a new logs model
func NewLogsModel() *LogsModel {
	return &LogsModel{
		maxLines: 50,
		logType:  "main",
		loading:  true,
	}
}

// Load reads log files
func (m *LogsModel) Load() {
	home, _ := os.UserHomeDir()
	var logPath string
	
	if m.logType == "session" {
		logPath = filepath.Join(home, ".cache", "wizado", "latest-session.log")
	} else {
		logPath = filepath.Join(home, ".cache", "wizado", "wizado.log")
	}
	
	data, err := os.ReadFile(logPath)
	if err != nil {
		m.lines = []string{"No logs found: " + logPath}
		m.loading = false
		return
	}
	
	allLines := strings.Split(string(data), "\n")
	
	// Get last N lines
	start := 0
	if len(allLines) > m.maxLines {
		start = len(allLines) - m.maxLines
	}
	m.lines = allLines[start:]
	m.loading = false
}

func (m Model) updateLogs(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.logsModel.scroll > 0 {
				m.logsModel.scroll--
			}
		case "down", "j":
			maxScroll := len(m.logsModel.lines) - 20 // visible lines
			if maxScroll < 0 {
				maxScroll = 0
			}
			if m.logsModel.scroll < maxScroll {
				m.logsModel.scroll++
			}
		case "tab":
			// Toggle between main and session logs
			if m.logsModel.logType == "main" {
				m.logsModel.logType = "session"
			} else {
				m.logsModel.logType = "main"
			}
			m.logsModel.scroll = 0
			m.logsModel.Load()
		case "esc", "q":
			m.screen = ScreenMain
			return m, nil
		}
	}
	return m, nil
}

func (m Model) viewLogs() string {
	var b strings.Builder

	// Title
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		Render("LOGS")
	b.WriteString(title)
	b.WriteString("\n")
	
	// Log type indicator
	mainStyle := normalStyle
	sessionStyle := normalStyle
	if m.logsModel.logType == "main" {
		mainStyle = selectedStyle
	} else {
		sessionStyle = selectedStyle
	}
	b.WriteString(mainStyle.Render("[Main]"))
	b.WriteString(" ")
	b.WriteString(sessionStyle.Render("[Session]"))
	b.WriteString("\n\n")

	if m.logsModel == nil || m.logsModel.loading {
		b.WriteString("Loading logs...")
		return boxStyle.Render(b.String())
	}

	if len(m.logsModel.lines) == 0 {
		b.WriteString("No logs found.")
		return boxStyle.Render(b.String())
	}

	// Display visible lines
	logStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Width(60)
	
	visibleLines := 15
	start := m.logsModel.scroll
	end := start + visibleLines
	if end > len(m.logsModel.lines) {
		end = len(m.logsModel.lines)
	}
	
	for i := start; i < end; i++ {
		line := m.logsModel.lines[i]
		if len(line) > 60 {
			line = line[:57] + "..."
		}
		// Color based on log level
		if strings.Contains(line, "[ERROR]") {
			b.WriteString(errorStyle.Render(line))
		} else if strings.Contains(line, "[WARN]") {
			b.WriteString(warningStyle.Render(line))
		} else {
			b.WriteString(logStyle.Render(line))
		}
		b.WriteString("\n")
	}

	// Scroll indicator
	if len(m.logsModel.lines) > visibleLines {
		scrollInfo := lipgloss.NewStyle().Foreground(mutedColor).Render(
			" (showing " + string(rune('0'+start)) + "-" + string(rune('0'+end)) + " of " + string(rune('0'+len(m.logsModel.lines))) + ")")
		b.WriteString(scrollInfo)
	}

	// Help
	b.WriteString(helpStyle.Render("\n↑/↓: scroll • tab: switch logs • esc: back"))

	return boxStyle.Render(b.String())
}

