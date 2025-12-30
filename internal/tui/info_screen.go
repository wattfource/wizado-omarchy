package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/wattfource/wizado/internal/license"
	"github.com/wattfource/wizado/internal/sysinfo"
)

// SystemInfoModel holds system information for display
type SystemInfoModel struct {
	info    *sysinfo.SystemInfo
	loading bool
	scroll  int
}

// NewSystemInfoModel creates a new system info model
func NewSystemInfoModel() *SystemInfoModel {
	return &SystemInfoModel{
		loading: true,
	}
}

// Load collects system information
func (m *SystemInfoModel) Load(version string) {
	m.info = sysinfo.Collect(version)
	m.loading = false
}

// sysInfoLoadedMsg is sent when system info is loaded
type sysInfoLoadedMsg struct {
	info *sysinfo.SystemInfo
}

func (m Model) updateSystemInfo(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.sysInfo.scroll > 0 {
				m.sysInfo.scroll--
			}
		case "down", "j":
			m.sysInfo.scroll++
		case "esc", "q":
			m.screen = ScreenMain
			return m, nil
		}
	case sysInfoLoadedMsg:
		m.sysInfo.info = msg.info
		m.sysInfo.loading = false
	}
	return m, nil
}

func (m Model) viewSystemInfo() string {
	var b strings.Builder

	// Title
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		Render("SYSTEM INFORMATION")
	b.WriteString(title)
	b.WriteString("\n\n")

	if m.sysInfo == nil || m.sysInfo.loading || m.sysInfo.info == nil {
		b.WriteString("Loading system information...")
		return boxStyle.Render(b.String())
	}

	info := m.sysInfo.info

	// Hardware section
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(secondaryColor)
	labelStyle := lipgloss.NewStyle().Foreground(mutedColor)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))

	b.WriteString(sectionStyle.Render("Hardware"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s %s\n", labelStyle.Render("CPU:"), valueStyle.Render(info.CPU.Model)))
	b.WriteString(fmt.Sprintf("  %s %s\n", labelStyle.Render("GPU:"), valueStyle.Render(info.GPU.Primary)))
	if info.GPU.DriverVersion != "" {
		b.WriteString(fmt.Sprintf("  %s %s\n", labelStyle.Render("Driver:"), valueStyle.Render(info.GPU.DriverVersion)))
	}
	b.WriteString(fmt.Sprintf("  %s %d GiB\n", labelStyle.Render("RAM:"), info.Memory.TotalMiB/1024))
	if info.Display.Primary.Width > 0 {
		display := fmt.Sprintf("%dx%d @ %.0fHz", info.Display.Primary.Width, info.Display.Primary.Height, info.Display.Primary.RefreshHz)
		b.WriteString(fmt.Sprintf("  %s %s\n", labelStyle.Render("Display:"), valueStyle.Render(display)))
	}
	b.WriteString("\n")

	// Input Devices
	b.WriteString(sectionStyle.Render("Input Devices"))
	b.WriteString("\n")
	if info.Input.HasKeyboard {
		name := "detected"
		if len(info.Input.Keyboards) > 0 {
			name = info.Input.Keyboards[0].Name
		}
		b.WriteString(fmt.Sprintf("  %s %s\n", successStyle.Render("✓"), valueStyle.Render("Keyboard: "+name)))
	} else {
		b.WriteString(fmt.Sprintf("  %s %s\n", errorStyle.Render("✗"), "Keyboard: not detected"))
	}
	if info.Input.HasMouse {
		name := "detected"
		if len(info.Input.Mice) > 0 {
			name = info.Input.Mice[0].Name
		}
		b.WriteString(fmt.Sprintf("  %s %s\n", successStyle.Render("✓"), valueStyle.Render("Mouse: "+name)))
	} else {
		b.WriteString(fmt.Sprintf("  %s %s\n", errorStyle.Render("✗"), "Mouse: not detected"))
	}
	if info.Input.HasController {
		name := "detected"
		if len(info.Input.Controllers) > 0 {
			name = info.Input.Controllers[0].Name
		}
		b.WriteString(fmt.Sprintf("  %s %s\n", successStyle.Render("✓"), valueStyle.Render("Controller: "+name)))
	} else {
		b.WriteString(fmt.Sprintf("  %s %s\n", labelStyle.Render("○"), "Controller: not connected"))
	}
	b.WriteString("\n")

	// Software
	b.WriteString(sectionStyle.Render("Software"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s %s\n", labelStyle.Render("OS:"), valueStyle.Render(info.OS.Name)))
	b.WriteString(fmt.Sprintf("  %s %s\n", labelStyle.Render("Kernel:"), valueStyle.Render(info.OS.Kernel)))
	if info.Desktop.Compositor != "" {
		b.WriteString(fmt.Sprintf("  %s %s %s\n", labelStyle.Render("Desktop:"), valueStyle.Render(info.Desktop.Compositor), info.Desktop.Version))
	}
	b.WriteString("\n")

	// Dependencies
	b.WriteString(sectionStyle.Render("Dependencies"))
	b.WriteString("\n")
	printDep := func(name string, pkg sysinfo.PackageInfo) {
		if pkg.Installed {
			ver := pkg.Version
			if ver == "" {
				ver = "installed"
			}
			b.WriteString(fmt.Sprintf("  %s %s %s\n", successStyle.Render("✓"), valueStyle.Render(name+":"), ver))
		} else {
			b.WriteString(fmt.Sprintf("  %s %s\n", errorStyle.Render("✗"), name+": not installed"))
		}
	}
	printDep("Steam", info.Dependencies.Steam)
	printDep("Gamescope", info.Dependencies.Gamescope)
	printDep("GameMode", info.Dependencies.GameMode)
	printDep("MangoHUD", info.Dependencies.MangoHUD)
	printDep("Hyprland", info.Dependencies.Hyprland)
	b.WriteString("\n")

	// Network
	b.WriteString(sectionStyle.Render("Network"))
	b.WriteString("\n")
	if info.Network.HasInternet {
		connType := info.Network.ConnectionType
		if info.Network.SSID != "" {
			connType = "WiFi: " + info.Network.SSID
		}
		b.WriteString(fmt.Sprintf("  %s %s\n", successStyle.Render("✓"), valueStyle.Render("Internet: "+connType)))
	} else {
		b.WriteString(fmt.Sprintf("  %s %s\n", errorStyle.Render("✗"), "Internet: not connected"))
	}
	b.WriteString("\n")

	// License
	b.WriteString(sectionStyle.Render("License"))
	b.WriteString("\n")
	result := license.Check()
	switch result.Status {
	case license.StatusValid:
		b.WriteString(fmt.Sprintf("  %s\n", successStyle.Render("✓ Valid")))
	case license.StatusOfflineGrace:
		b.WriteString(fmt.Sprintf("  %s\n", warningStyle.Render("✓ Valid (offline)")))
	case license.StatusNoLicense:
		b.WriteString(fmt.Sprintf("  %s\n", errorStyle.Render("✗ Not activated")))
	default:
		b.WriteString(fmt.Sprintf("  %s %s\n", errorStyle.Render("✗"), string(result.Status)))
	}

	// Help
	b.WriteString(helpStyle.Render("\n↑/↓: scroll • esc: back"))

	return boxStyle.Render(b.String())
}

