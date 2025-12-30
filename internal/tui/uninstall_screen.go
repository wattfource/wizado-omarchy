package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// UninstallModel holds uninstall state
type UninstallModel struct {
	confirmed   bool
	executing   bool
	done        bool
	result      string
	cursor      int
}

// NewUninstallModel creates a new uninstall model
func NewUninstallModel() *UninstallModel {
	return &UninstallModel{
		cursor: 1, // Default to "No"
	}
}

func (m Model) updateUninstall(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.uninstallModel.done {
			// Any key returns to main
			m.screen = ScreenMain
			m.uninstallModel = NewUninstallModel()
			return m, nil
		}
		
		if m.uninstallModel.executing {
			return m, nil
		}
		
		switch msg.String() {
		case "left", "h":
			if m.uninstallModel.cursor > 0 {
				m.uninstallModel.cursor--
			}
		case "right", "l":
			if m.uninstallModel.cursor < 1 {
				m.uninstallModel.cursor++
			}
		case "enter", " ":
			if m.uninstallModel.cursor == 0 {
				// Yes - execute uninstall
				m.uninstallModel.executing = true
				m.uninstallModel.result = executeUninstall()
				m.uninstallModel.done = true
				m.uninstallModel.executing = false
			} else {
				// No - go back
				m.screen = ScreenMain
				m.uninstallModel = NewUninstallModel()
			}
			return m, nil
		case "esc", "q":
			m.screen = ScreenMain
			m.uninstallModel = NewUninstallModel()
			return m, nil
		}
	}
	return m, nil
}

func executeUninstall() string {
	var result strings.Builder
	home, _ := os.UserHomeDir()
	
	// Remove config directory
	configDir := filepath.Join(home, ".config", "wizado")
	if err := os.RemoveAll(configDir); err != nil {
		result.WriteString("Warning: could not remove config\n")
	} else {
		result.WriteString("Removed: " + configDir + "\n")
	}
	
	// Remove cache directory
	cacheDir := filepath.Join(home, ".cache", "wizado")
	if err := os.RemoveAll(cacheDir); err != nil {
		result.WriteString("Warning: could not remove cache\n")
	} else {
		result.WriteString("Removed: " + cacheDir + "\n")
	}
	
	// Remove local data directory (telemetry)
	dataDir := filepath.Join(home, ".local", "share", "wizado")
	if err := os.RemoveAll(dataDir); err != nil {
		result.WriteString("Warning: could not remove data\n")
	} else {
		result.WriteString("Removed: " + dataDir + "\n")
	}
	
	// Remove keybindings from Hyprland config
	bindingsPaths := []string{
		filepath.Join(home, ".config", "hypr", "bindings.conf"),
		filepath.Join(home, ".config", "hypr", "keybinds.conf"),
		filepath.Join(home, ".config", "hypr", "hyprland.conf"),
	}
	
	for _, path := range bindingsPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		
		content := string(data)
		if !strings.Contains(content, "# Wizado - added by wizado") {
			continue
		}
		
		// Remove wizado bindings block
		startMarker := "# Wizado - added by wizado"
		endMarker := "# End Wizado bindings"
		
		startIdx := strings.Index(content, startMarker)
		endIdx := strings.Index(content, endMarker)
		
		if startIdx != -1 && endIdx != -1 {
			// Include a newline before the block
			if startIdx > 0 && content[startIdx-1] == '\n' {
				startIdx--
			}
			content = content[:startIdx] + content[endIdx+len(endMarker):]
			if err := os.WriteFile(path, []byte(content), 0644); err == nil {
				result.WriteString("Removed keybindings from: " + path + "\n")
			}
		}
	}
	
	// Reload Hyprland
	exec.Command("hyprctl", "reload").Run()
	
	result.WriteString("\nWizado configuration removed.\n")
	result.WriteString("To fully uninstall, run:\n")
	result.WriteString("  sudo pacman -R wizado\n")
	
	return result.String()
}

func (m Model) viewUninstall() string {
	var b strings.Builder

	// Title
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(errorColor).
		Render("UNINSTALL WIZADO")
	b.WriteString(title)
	b.WriteString("\n\n")

	if m.uninstallModel.done {
		b.WriteString(successStyle.Render("Uninstall completed:"))
		b.WriteString("\n\n")
		b.WriteString(m.uninstallModel.result)
		b.WriteString(helpStyle.Render("\nPress any key to return..."))
		return boxStyle.Render(b.String())
	}
	
	if m.uninstallModel.executing {
		b.WriteString("Removing wizado configuration...")
		return boxStyle.Render(b.String())
	}

	// Warning
	warningBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(warningColor).
		Padding(0, 1).
		Render("This will remove:\n• Configuration (~/.config/wizado)\n• Cache & logs (~/.cache/wizado)\n• Telemetry data (~/.local/share/wizado)\n• Hyprland keybindings")
	b.WriteString(warningBox)
	b.WriteString("\n\n")
	
	b.WriteString("Are you sure you want to uninstall?\n\n")
	
	// Yes/No buttons
	yesStyle := normalStyle
	noStyle := normalStyle
	if m.uninstallModel.cursor == 0 {
		yesStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255")).
			Background(errorColor).
			Padding(0, 2)
	} else {
		yesStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Padding(0, 2)
	}
	if m.uninstallModel.cursor == 1 {
		noStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255")).
			Background(successColor).
			Padding(0, 2)
	} else {
		noStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Padding(0, 2)
	}
	
	b.WriteString("  ")
	b.WriteString(yesStyle.Render("Yes, uninstall"))
	b.WriteString("  ")
	b.WriteString(noStyle.Render("No, go back"))
	b.WriteString("\n")

	// Help
	b.WriteString(helpStyle.Render("\n←/→: select • enter: confirm • esc: cancel"))

	return boxStyle.Render(b.String())
}

