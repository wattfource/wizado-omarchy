package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/wattfource/wizado/internal/license"
)

func (m Model) updateMain(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.menuItems)-1 {
				m.cursor++
			}
		case "enter", " ":
			return m.selectMenuItem()
		}
	}
	return m, nil
}

func (m Model) selectMenuItem() (tea.Model, tea.Cmd) {
	switch m.cursor {
	case 0: // Launch Steam
		// Check license first
		result := license.Check()
		if result.Status == license.StatusValid || result.Status == license.StatusOfflineGrace {
			m.launchSteam = true
			m.quitting = true
			return m, tea.Quit
		}
		// No valid license, show license entry
		m.screen = ScreenLicenseEntry
		m.emailInput.Focus()
		m.message = "License required to launch Steam"
		m.messageStyle = warningStyle
		return m, nil
		
	case 1: // License
		result := license.Check()
		if result.Status == license.StatusValid || result.Status == license.StatusOfflineGrace {
			m.screen = ScreenLicenseStatus
			m.licenseStatus = string(result.Status)
			if result.License != nil {
				m.licenseEmail = result.License.Email
			}
		} else {
			m.screen = ScreenLicenseEntry
			m.emailInput.Focus()
		}
		return m, nil
		
	case 2: // Settings
		m.screen = ScreenSettings
		return m, nil
		
	case 3: // System Info
		m.screen = ScreenSystemInfo
		if m.sysInfo == nil {
			m.sysInfo = NewSystemInfoModel()
		}
		m.sysInfo.Load("1.0.1") // TODO: pass actual version
		return m, nil
		
	case 4: // Logs
		m.screen = ScreenLogs
		if m.logsModel == nil {
			m.logsModel = NewLogsModel()
		}
		m.logsModel.Load()
		return m, nil
		
	case 5: // Uninstall
		m.screen = ScreenUninstall
		if m.uninstallModel == nil {
			m.uninstallModel = NewUninstallModel()
		}
		return m, nil
		
	case 6: // Exit
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) viewMain() string {
	var b strings.Builder
	
	// Banner
	banner := lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		Render("WIZADO")
	
	subtitle := subtitleStyle.Render("Steam Gaming Mode for Hyprland")
	
	b.WriteString(banner)
	b.WriteString("\n")
	b.WriteString(subtitle)
	b.WriteString("\n\n")
	
	// License status indicator
	result := license.Check()
	statusLine := m.formatLicenseStatus(result.Status)
	b.WriteString(statusLine)
	b.WriteString("\n\n")
	
	// Menu
	for i, item := range m.menuItems {
		cursor := "  "
		style := normalStyle
		if i == m.cursor {
			cursor = "▸ "
			style = selectedStyle
		}
		b.WriteString(cursor + style.Render(item) + "\n")
	}
	
	// Help
	b.WriteString(helpStyle.Render("\n↑/↓: navigate • enter: select • q: quit"))
	
	return boxStyle.Render(b.String())
}

func (m Model) formatLicenseStatus(status license.Status) string {
	switch status {
	case license.StatusValid:
		return successStyle.Render("Licensed")
	case license.StatusOfflineGrace:
		return warningStyle.Render("Licensed (offline)")
	case license.StatusNoLicense:
		return errorStyle.Render("No license")
	case license.StatusInvalid:
		return errorStyle.Render("Invalid license")
	case license.StatusExpired:
		return errorStyle.Render("License expired")
	case license.StatusMachineMismatch:
		return warningStyle.Render("Wrong machine")
	case license.StatusOfflineExpired:
		return errorStyle.Render("Offline expired")
	case license.StatusTampered:
		return errorStyle.Render("Tampered")
	case license.StatusClockTampered:
		return errorStyle.Render("Clock error")
	default:
		return lipgloss.NewStyle().Foreground(mutedColor).Render(fmt.Sprintf("%s", status))
	}
}

