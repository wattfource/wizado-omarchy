// Package tui provides the terminal user interface for wizado
package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Screen represents different TUI screens
type Screen int

const (
	ScreenMain Screen = iota
	ScreenLicenseEntry
	ScreenLicenseStatus
	ScreenSettings
)

// Colors
var (
	primaryColor   = lipgloss.Color("212") // Pink/magenta
	secondaryColor = lipgloss.Color("39")  // Cyan
	successColor   = lipgloss.Color("82")  // Green
	errorColor     = lipgloss.Color("196") // Red
	warningColor   = lipgloss.Color("214") // Orange
	mutedColor     = lipgloss.Color("245") // Gray
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Italic(true)

	selectedStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250"))

	successStyle = lipgloss.NewStyle().
			Foreground(successColor).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	warningStyle = lipgloss.NewStyle().
			Foreground(warningColor)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(1, 2)

	helpStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			MarginTop(1)
)

// Model represents the TUI state
type Model struct {
	screen       Screen
	cursor       int
	menuItems    []string
	width        int
	height       int
	
	// License entry
	emailInput   textinput.Model
	keyInput     textinput.Model
	focusIndex   int
	
	// Messages
	message      string
	messageStyle lipgloss.Style
	
	// License status
	licenseStatus string
	licenseEmail  string
	
	// Settings
	settings     *SettingsModel
	
	// Should quit
	quitting     bool
	
	// Should launch Steam after
	launchSteam  bool
}

// NewModel creates a new TUI model
func NewModel() Model {
	emailInput := textinput.New()
	emailInput.Placeholder = "your@email.com"
	emailInput.CharLimit = 100
	emailInput.Width = 40
	
	keyInput := textinput.New()
	keyInput.Placeholder = "XXXX-XXXX-XXXX"
	keyInput.CharLimit = 20
	keyInput.Width = 20
	
	return Model{
		screen:    ScreenMain,
		menuItems: []string{
			"Launch Steam",
			"License",
			"Settings",
			"Exit",
		},
		emailInput:   emailInput,
		keyInput:     keyInput,
		settings:     NewSettingsModel(),
		messageStyle: normalStyle,
	}
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.screen == ScreenMain {
				m.quitting = true
				return m, tea.Quit
			}
			// Go back to main screen
			m.screen = ScreenMain
			m.message = ""
			return m, nil
			
		case "esc":
			if m.screen != ScreenMain {
				m.screen = ScreenMain
				m.message = ""
				return m, nil
			}
		}
	
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	}
	
	// Delegate to screen-specific update
	switch m.screen {
	case ScreenMain:
		return m.updateMain(msg)
	case ScreenLicenseEntry:
		return m.updateLicenseEntry(msg)
	case ScreenLicenseStatus:
		return m.updateLicenseStatus(msg)
	case ScreenSettings:
		return m.updateSettings(msg)
	}
	
	return m, nil
}

// View implements tea.Model
func (m Model) View() string {
	if m.quitting {
		return ""
	}
	
	switch m.screen {
	case ScreenMain:
		return m.viewMain()
	case ScreenLicenseEntry:
		return m.viewLicenseEntry()
	case ScreenLicenseStatus:
		return m.viewLicenseStatus()
	case ScreenSettings:
		return m.viewSettings()
	}
	
	return ""
}

// ShouldLaunchSteam returns true if Steam should be launched after TUI exits
func (m Model) ShouldLaunchSteam() bool {
	return m.launchSteam
}

// Run starts the TUI
func Run() (launchSteam bool, err error) {
	m := NewModel()
	p := tea.NewProgram(m, tea.WithAltScreen())
	
	finalModel, err := p.Run()
	if err != nil {
		return false, err
	}
	
	if fm, ok := finalModel.(Model); ok {
		return fm.ShouldLaunchSteam(), nil
	}
	
	return false, nil
}

// RunLicensePrompt shows the license entry screen directly
func RunLicensePrompt() error {
	m := NewModel()
	m.screen = ScreenLicenseEntry
	m.emailInput.Focus()
	
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
