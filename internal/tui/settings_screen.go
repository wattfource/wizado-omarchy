package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wattfource/wizado/internal/config"
)

// SettingsModel handles settings editing
type SettingsModel struct {
	cfg         *config.Config
	cursor      int
	editing     bool
	editCursor  int
	editOptions []string
}

// NewSettingsModel creates a new settings model
func NewSettingsModel() *SettingsModel {
	cfg, _ := config.Load()
	return &SettingsModel{
		cfg: cfg,
	}
}

var settingsFields = []string{
	"Resolution",
	"FSR Upscaling",
	"Frame Limit",
	"VRR/Adaptive Sync",
	"MangoHUD",
	"Steam UI",
	"Workspace",
	"─────────────",
	"Save & Exit",
	"Cancel",
}

func (m Model) updateSettings(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.settings.editing {
			return m.updateSettingsEdit(msg)
		}
		
		switch msg.String() {
		case "up", "k":
			if m.settings.cursor > 0 {
				m.settings.cursor--
				// Skip separator
				if m.settings.cursor == 7 {
					m.settings.cursor--
				}
			}
		case "down", "j":
			if m.settings.cursor < len(settingsFields)-1 {
				m.settings.cursor++
				// Skip separator
				if m.settings.cursor == 7 {
					m.settings.cursor++
				}
			}
		case "enter", " ":
			return m.selectSettingsItem()
		}
	}
	return m, nil
}

func (m Model) updateSettingsEdit(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.settings.editCursor > 0 {
				m.settings.editCursor--
			}
		case "down", "j":
			if m.settings.editCursor < len(m.settings.editOptions)-1 {
				m.settings.editCursor++
			}
		case "enter", " ":
			m.applySettingsEdit()
			m.settings.editing = false
			return m, nil
		case "esc":
			m.settings.editing = false
			return m, nil
		}
	}
	return m, nil
}

func (m Model) selectSettingsItem() (tea.Model, tea.Cmd) {
	switch m.settings.cursor {
	case 0: // Resolution
		m.settings.editing = true
		m.settings.editOptions = []string{"auto", "1920x1080", "2560x1440", "3840x2160"}
		m.settings.editCursor = 0
	case 1: // FSR
		m.settings.editing = true
		m.settings.editOptions = config.FSROptions()
		m.settings.editCursor = 0
	case 2: // Frame Limit
		m.settings.editing = true
		m.settings.editOptions = make([]string, len(config.FrameLimitOptions()))
		for i, v := range config.FrameLimitOptions() {
			if v == 0 {
				m.settings.editOptions[i] = "unlimited"
			} else {
				m.settings.editOptions[i] = fmt.Sprintf("%d", v)
			}
		}
		m.settings.editCursor = 0
	case 3: // VRR
		m.settings.editing = true
		m.settings.editOptions = []string{"off", "on"}
		m.settings.editCursor = 0
	case 4: // MangoHUD
		m.settings.editing = true
		m.settings.editOptions = []string{"off", "on"}
		m.settings.editCursor = 0
	case 5: // Steam UI
		m.settings.editing = true
		m.settings.editOptions = config.SteamUIOptions()
		m.settings.editCursor = 0
	case 6: // Workspace
		m.settings.editing = true
		m.settings.editOptions = make([]string, len(config.WorkspaceOptions()))
		for i, v := range config.WorkspaceOptions() {
			m.settings.editOptions[i] = fmt.Sprintf("%d", v)
		}
		m.settings.editCursor = 0
	case 8: // Save & Exit
		config.Save(m.settings.cfg)
		m.screen = ScreenMain
		return m, nil
	case 9: // Cancel
		m.settings.cfg, _ = config.Load() // Reload to discard changes
		m.screen = ScreenMain
		return m, nil
	}
	return m, nil
}

func (m *Model) applySettingsEdit() {
	selected := m.settings.editOptions[m.settings.editCursor]
	
	switch m.settings.cursor {
	case 0: // Resolution
		m.settings.cfg.Resolution = selected
	case 1: // FSR
		m.settings.cfg.FSR = selected
	case 2: // Frame Limit
		if selected == "unlimited" {
			m.settings.cfg.FrameLimit = 0
		} else {
			fmt.Sscanf(selected, "%d", &m.settings.cfg.FrameLimit)
		}
	case 3: // VRR
		m.settings.cfg.VRR = selected == "on"
	case 4: // MangoHUD
		m.settings.cfg.MangoHUD = selected == "on"
	case 5: // Steam UI
		m.settings.cfg.SteamUI = selected
	case 6: // Workspace
		fmt.Sscanf(selected, "%d", &m.settings.cfg.Workspace)
	}
}

func (m Model) viewSettings() string {
	var b strings.Builder
	
	title := titleStyle.Render("Settings")
	b.WriteString(title)
	b.WriteString("\n\n")
	
	cfg := m.settings.cfg
	values := []string{
		cfg.Resolution,
		cfg.FSR,
		fmt.Sprintf("%d", cfg.FrameLimit),
		boolToOnOff(cfg.VRR),
		boolToOnOff(cfg.MangoHUD),
		cfg.SteamUI,
		fmt.Sprintf("%d", cfg.Workspace),
		"",
		"",
		"",
	}
	
	for i, field := range settingsFields {
		cursor := "  "
		style := normalStyle
		
		if i == m.settings.cursor {
			cursor = "▸ "
			style = selectedStyle
		}
		
		if i == 7 {
			// Separator
			b.WriteString(mutedColor.Render("  " + field) + "\n")
			continue
		}
		
		if i < 7 {
			line := fmt.Sprintf("%s%-18s %s", cursor, field+":", values[i])
			b.WriteString(style.Render(line) + "\n")
		} else {
			b.WriteString(cursor + style.Render(field) + "\n")
		}
	}
	
	// Edit popup
	if m.settings.editing {
		b.WriteString("\n")
		b.WriteString(boxStyle.Render(m.viewEditPopup()))
	}
	
	// Help
	b.WriteString(helpStyle.Render("\n↑/↓: navigate • enter: edit • esc: back"))
	
	return boxStyle.Render(b.String())
}

func (m Model) viewEditPopup() string {
	var b strings.Builder
	
	b.WriteString("Select value:\n\n")
	
	for i, opt := range m.settings.editOptions {
		cursor := "  "
		style := normalStyle
		
		if i == m.settings.editCursor {
			cursor = "▸ "
			style = selectedStyle
		}
		
		b.WriteString(cursor + style.Render(opt) + "\n")
	}
	
	return b.String()
}

func boolToOnOff(v bool) string {
	if v {
		return "on"
	}
	return "off"
}

