package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/wattfource/wizado/internal/license"
)

// License entry messages
type activationMsg struct {
	result *license.ActivationResult
	err    error
}

func (m Model) updateLicenseEntry(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "shift+tab", "up", "down":
			// Toggle between email and key inputs
			if m.focusIndex == 0 {
				m.focusIndex = 1
				m.emailInput.Blur()
				m.keyInput.Focus()
			} else {
				m.focusIndex = 0
				m.keyInput.Blur()
				m.emailInput.Focus()
			}
			return m, nil
			
		case "enter":
			if m.emailInput.Value() == "" || m.keyInput.Value() == "" {
				m.message = "Please enter both email and license key"
				m.messageStyle = errorStyle
				return m, nil
			}
			
			// Attempt activation
			m.message = "Activating license..."
			m.messageStyle = normalStyle
			
			return m, func() tea.Msg {
				result, err := license.Activate(m.emailInput.Value(), m.keyInput.Value())
				return activationMsg{result: result, err: err}
			}
		}
		
	case activationMsg:
		if msg.err != nil {
			m.message = fmt.Sprintf("Activation failed: %v", msg.err)
			m.messageStyle = errorStyle
			return m, nil
		}
		
		if msg.result.Success {
			m.message = fmt.Sprintf("✓ License activated! (%d/%d slots used)", 
				msg.result.SlotsUsed, msg.result.SlotsTotal)
			m.messageStyle = successStyle
			// Switch to main screen after short delay
			m.screen = ScreenMain
			return m, nil
		}
		
		m.message = msg.result.Message
		m.messageStyle = errorStyle
		return m, nil
	}
	
	// Update focused input
	var cmd tea.Cmd
	if m.focusIndex == 0 {
		m.emailInput, cmd = m.emailInput.Update(msg)
	} else {
		m.keyInput, cmd = m.keyInput.Update(msg)
	}
	
	return m, cmd
}

func (m Model) viewLicenseEntry() string {
	var b strings.Builder
	
	title := titleStyle.Render("Enter License")
	b.WriteString(title)
	b.WriteString("\n\n")
	
	// Email input
	emailLabel := "Email: "
	if m.focusIndex == 0 {
		emailLabel = selectedStyle.Render("▸ Email: ")
	}
	b.WriteString(emailLabel)
	b.WriteString(m.emailInput.View())
	b.WriteString("\n\n")
	
	// Key input
	keyLabel := "License Key: "
	if m.focusIndex == 1 {
		keyLabel = selectedStyle.Render("▸ License Key: ")
	}
	b.WriteString(keyLabel)
	b.WriteString(m.keyInput.View())
	b.WriteString("\n\n")
	
	// Message
	if m.message != "" {
		b.WriteString(m.messageStyle.Render(m.message))
		b.WriteString("\n")
	}
	
	// Purchase link
	b.WriteString("\n")
	purchaseStyle := lipgloss.NewStyle().Foreground(secondaryColor).Italic(true)
	b.WriteString(purchaseStyle.Render("$10 for 5 machines at wizado.app"))
	
	// Help
	b.WriteString(helpStyle.Render("\n\ntab: switch field • enter: activate • esc: back"))
	
	return boxStyle.Render(b.String())
}

func (m Model) updateLicenseStatus(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "c":
			// Clear license
			license.Clear()
			m.screen = ScreenMain
			return m, nil
		case "r":
			// Re-enter license
			m.screen = ScreenLicenseEntry
			m.emailInput.Focus()
			m.emailInput.SetValue("")
			m.keyInput.SetValue("")
			return m, nil
		}
	}
	return m, nil
}

func (m Model) viewLicenseStatus() string {
	var b strings.Builder
	
	title := titleStyle.Render("License Status")
	b.WriteString(title)
	b.WriteString("\n\n")
	
	// Status
	result := license.Check()
	statusLine := m.formatLicenseStatus(result.Status)
	b.WriteString(fmt.Sprintf("Status: %s\n", statusLine))
	
	// Email
	if result.License != nil && result.License.Email != "" {
		b.WriteString(fmt.Sprintf("Email: %s\n", result.License.Email))
		
		// Masked key
		key := result.License.Key
		if len(key) > 8 {
			masked := key[:4] + "****" + key[len(key)-4:]
			b.WriteString(fmt.Sprintf("Key: %s\n", masked))
		}
		
		// Last verified
		b.WriteString(fmt.Sprintf("Last Verified: %s\n", 
			result.License.LastVerified.Format("2006-01-02 15:04")))
	}
	
	// Help
	b.WriteString(helpStyle.Render("\n\nc: clear license • r: re-enter • esc: back"))
	
	return boxStyle.Render(b.String())
}

