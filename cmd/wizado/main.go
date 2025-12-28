// wizado: Steam gaming launcher for Hyprland
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wattfource/wizado/internal/config"
	"github.com/wattfource/wizado/internal/launcher"
	"github.com/wattfource/wizado/internal/license"
	"github.com/wattfource/wizado/internal/logging"
	"github.com/wattfource/wizado/internal/setup"
	"github.com/wattfource/wizado/internal/sysinfo"
	"github.com/wattfource/wizado/internal/telemetry"
	"github.com/wattfource/wizado/internal/tui"
)

// Version is set at build time
var Version = "dev"

func main() {
	// Initialize logging
	logCfg := logging.DefaultConfig()
	logCfg.Component = "wizado"
	logging.Init(logCfg)
	
	// Initialize telemetry
	telemetryCfg := telemetry.DefaultConfig()
	telemetryCfg.Version = Version
	telemetry.Init(telemetryCfg)
	
	rootCmd := &cobra.Command{
		Use:   "wizado",
		Short: "Steam gaming launcher for Hyprland",
		Long: `Wizado is a Steam gaming launcher for Hyprland on Arch Linux.
It provides gamescope integration, FSR upscaling, GameMode support,
and per-game configurations optimized for desktop gaming.

License required: $10 for 5 machines at https://wizado.app`,
		Version: Version,
		Run:     runLaunch,
	}

	// Config command
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Open settings and license configuration",
		Run:   runConfig,
	}

	// Setup command
	setupCmd := &cobra.Command{
		Use:   "setup",
		Short: "Install dependencies and configure system",
		Run:   runSetup,
	}
	setupCmd.Flags().BoolP("yes", "y", false, "Non-interactive mode")
	setupCmd.Flags().Bool("dry-run", false, "Print what would be done without making changes")

	// Status command (for waybar)
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Output license status as JSON (for waybar)",
		Run:   runStatus,
	}

	// Activate command
	activateCmd := &cobra.Command{
		Use:   "activate EMAIL KEY",
		Short: "Activate a license (non-interactive)",
		Args:  cobra.ExactArgs(2),
		Run:   runActivate,
	}

	// Remove command
	removeCmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove wizado configuration and keybindings",
		Run:   runRemove,
	}
	removeCmd.Flags().BoolP("yes", "y", false, "Skip confirmation")

	// Info command - new!
	infoCmd := &cobra.Command{
		Use:   "info",
		Short: "Display system information and diagnostics",
		Run:   runInfo,
	}
	infoCmd.Flags().Bool("json", false, "Output as JSON")
	
	// Logs command - new!
	logsCmd := &cobra.Command{
		Use:   "logs",
		Short: "View or manage logs",
		Run:   runLogs,
	}
	logsCmd.Flags().BoolP("follow", "f", false, "Follow log output")
	logsCmd.Flags().Bool("session", false, "View latest session log")
	logsCmd.Flags().Bool("clear", false, "Clear all logs")

	rootCmd.AddCommand(configCmd, setupCmd, statusCmd, activateCmd, removeCmd, infoCmd, logsCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runLaunch(cmd *cobra.Command, args []string) {
	log := logging.WithComponent("main")
	log.Info("Wizado launch initiated")
	
	// Collect system info for telemetry
	sysInfo := launcher.CollectSystemInfo(Version)
	
	// Check license
	result := license.Check()
	
	if result.Status != license.StatusValid && result.Status != license.StatusOfflineGrace {
		log.Info("License check failed, launching TUI")
		
		// Need license - run TUI
		launchSteam, err := tui.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			log.Errorf("TUI error: %v", err)
			os.Exit(1)
		}
		
		if !launchSteam {
			log.Info("User cancelled launch")
			os.Exit(0)
		}
		
		// Re-check license after TUI
		result = license.Check()
		if result.Status != license.StatusValid && result.Status != license.StatusOfflineGrace {
			fmt.Fprintln(os.Stderr, "License required to launch Steam")
			log.Error("License required after TUI")
			os.Exit(1)
		}
	}
	
	log.Info("License valid, proceeding with launch")
	
	// Load config and launch
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		log.Errorf("Config load error: %v", err)
		os.Exit(1)
	}
	
	// Log system info
	log.WithFields(map[string]any{
		"gpu":         sysInfo.GPU.Primary,
		"cpu":         sysInfo.CPU.Model,
		"ram_gib":     sysInfo.Memory.TotalMiB / 1024,
		"resolution":  fmt.Sprintf("%dx%d", sysInfo.Display.Primary.Width, sysInfo.Display.Primary.Height),
		"controller":  sysInfo.Input.HasController,
	}).Info("System configuration")
	
	if err := launcher.Launch(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Launch failed: %v\n", err)
		log.Errorf("Launch failed: %v", err)
		telemetry.RecordError("launcher", err.Error(), nil)
		os.Exit(1)
	}
}

func runConfig(cmd *cobra.Command, args []string) {
	if _, err := tui.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runSetup(cmd *cobra.Command, args []string) {
	nonInteractive, _ := cmd.Flags().GetBool("yes")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	
	opts := setup.Options{
		NonInteractive: nonInteractive,
		DryRun:         dryRun,
	}
	
	if err := setup.Run(opts); err != nil {
		fmt.Fprintf(os.Stderr, "Setup failed: %v\n", err)
		telemetry.RecordError("setup", err.Error(), nil)
		os.Exit(1)
	}
	
	telemetry.RecordEvent(telemetry.EventSetup, map[string]any{
		"success": true,
	})
}

func runStatus(cmd *cobra.Command, args []string) {
	// Output JSON for waybar
	result := license.Check()
	
	icon := "\uef01" // nf-fa-hat_wizard
	tooltip := "Wizado Gaming Mode"
	class := "inactive"
	alt := "unlicensed"
	
	if result.Status == license.StatusValid || result.Status == license.StatusOfflineGrace {
		class = "licensed"
		alt = "licensed"
		tooltip = "Wizado - Steam Gaming Mode\\n━━━━━━━━━━━━━━━━━━━━━━━\\n✓ Licensed\\n\\nLeft-click: Launch Steam\\nRight-click: Menu"
	} else {
		class = "unlicensed"
		tooltip = "Wizado - Steam Gaming Mode\\n━━━━━━━━━━━━━━━━━━━━━━━\\n✗ License Required\\n$10 for 5 machines\\nwizado.app\\n\\nLeft-click: Launch Steam\\nRight-click: Menu"
	}
	
	output := map[string]string{
		"text":    icon,
		"tooltip": tooltip,
		"class":   class,
		"alt":     alt,
	}
	
	jsonData, _ := json.Marshal(output)
	fmt.Println(string(jsonData))
}

func runActivate(cmd *cobra.Command, args []string) {
	email := args[0]
	key := args[1]
	
	result, err := license.Activate(email, key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Activation failed: %v\n", err)
		os.Exit(1)
	}
	
	if result.Success {
		fmt.Printf("License activated successfully!\n")
		fmt.Printf("Email: %s\n", result.Email)
		fmt.Printf("Slots: %d/%d\n", result.SlotsUsed, result.SlotsTotal)
	} else {
		fmt.Fprintf(os.Stderr, "Activation failed: %s\n", result.Message)
		os.Exit(1)
	}
}

func runRemove(cmd *cobra.Command, args []string) {
	skipConfirm, _ := cmd.Flags().GetBool("yes")
	
	if !skipConfirm {
		fmt.Print("Remove wizado configuration and keybindings? [y/N]: ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" {
			fmt.Println("Cancelled")
			return
		}
	}
	
	home, _ := os.UserHomeDir()
	
	// Remove config directory
	configDir := filepath.Join(home, ".config", "wizado")
	if err := os.RemoveAll(configDir); err != nil {
		fmt.Printf("Warning: could not remove config: %v\n", err)
	} else {
		fmt.Printf("Removed: %s\n", configDir)
	}
	
	// Remove cache directory
	cacheDir := filepath.Join(home, ".cache", "wizado")
	if err := os.RemoveAll(cacheDir); err != nil {
		fmt.Printf("Warning: could not remove cache: %v\n", err)
	} else {
		fmt.Printf("Removed: %s\n", cacheDir)
	}
	
	// Remove local data directory (telemetry)
	dataDir := filepath.Join(home, ".local", "share", "wizado")
	if err := os.RemoveAll(dataDir); err != nil {
		fmt.Printf("Warning: could not remove data: %v\n", err)
	} else {
		fmt.Printf("Removed: %s\n", dataDir)
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
				fmt.Printf("Removed keybindings from: %s\n", path)
			}
		}
	}
	
	// Reload Hyprland
	exec.Command("hyprctl", "reload").Run()
	
	fmt.Println("Wizado removed successfully")
}

func runInfo(cmd *cobra.Command, args []string) {
	jsonOutput, _ := cmd.Flags().GetBool("json")
	
	fmt.Println("Collecting system information...")
	info := sysinfo.Collect(Version)
	
	if jsonOutput {
		data, _ := info.ToJSON()
		fmt.Println(string(data))
		return
	}
	
	// Print formatted summary
	fmt.Print(info.Summary())
	
	// Additional wizado-specific info
	fmt.Println("\nWizado Status")
	fmt.Println("═════════════")
	fmt.Printf("  Version: %s\n", Version)
	
	// License status
	result := license.Check()
	switch result.Status {
	case license.StatusValid:
		fmt.Println("  License: ✓ Valid")
	case license.StatusOfflineGrace:
		fmt.Println("  License: ✓ Valid (offline mode)")
	case license.StatusNoLicense:
		fmt.Println("  License: ✗ Not activated")
	default:
		fmt.Printf("  License: ✗ %s\n", result.Status)
	}
	
	// Log location
	home, _ := os.UserHomeDir()
	fmt.Printf("  Log file: %s\n", filepath.Join(home, ".cache", "wizado", "wizado.log"))
	
	// Telemetry status
	stats, _ := telemetry.Default().GetStats()
	fmt.Printf("  Telemetry: %v events recorded\n", stats["event_count"])
}

func runLogs(cmd *cobra.Command, args []string) {
	follow, _ := cmd.Flags().GetBool("follow")
	session, _ := cmd.Flags().GetBool("session")
	clear, _ := cmd.Flags().GetBool("clear")
	
	home, _ := os.UserHomeDir()
	
	if clear {
		logDir := filepath.Join(home, ".cache", "wizado")
		sessionsDir := filepath.Join(logDir, "sessions")
		
		os.Remove(filepath.Join(logDir, "wizado.log"))
		os.Remove(filepath.Join(logDir, "wizado.log.1"))
		os.Remove(filepath.Join(logDir, "wizado.log.2"))
		os.Remove(filepath.Join(logDir, "wizado.log.3"))
		os.Remove(filepath.Join(logDir, "wizado.log.4"))
		os.Remove(filepath.Join(logDir, "wizado.log.5"))
		os.RemoveAll(sessionsDir)
		
		fmt.Println("Logs cleared")
		return
	}
	
	var logFile string
	if session {
		logFile = filepath.Join(home, ".cache", "wizado", "latest-session.log")
	} else {
		logFile = filepath.Join(home, ".cache", "wizado", "wizado.log")
	}
	
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		fmt.Printf("Log file not found: %s\n", logFile)
		return
	}
	
	if follow {
		// Use tail -f
		tailCmd := exec.Command("tail", "-f", logFile)
		tailCmd.Stdout = os.Stdout
		tailCmd.Stderr = os.Stderr
		tailCmd.Run()
	} else {
		// Just cat the file
		data, err := os.ReadFile(logFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading log: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(string(data))
	}
}
