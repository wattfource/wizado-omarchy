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
	"github.com/wattfource/wizado/internal/setup"
	"github.com/wattfource/wizado/internal/tui"
)

// Version is set at build time
var Version = "dev"

func main() {
	rootCmd := &cobra.Command{
		Use:   "wizado",
		Short: "Steam gaming launcher for Hyprland",
		Long: `Wizado is a Steam gaming launcher for Hyprland on Arch Linux.
It provides gamescope integration, FSR upscaling, and per-game configurations.

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

	rootCmd.AddCommand(configCmd, setupCmd, statusCmd, activateCmd, removeCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runLaunch(cmd *cobra.Command, args []string) {
	// Check license
	result := license.Check()
	
	if result.Status != license.StatusValid && result.Status != license.StatusOfflineGrace {
		// Need license - run TUI
		launchSteam, err := tui.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		
		if !launchSteam {
			os.Exit(0)
		}
		
		// Re-check license after TUI
		result = license.Check()
		if result.Status != license.StatusValid && result.Status != license.StatusOfflineGrace {
			fmt.Fprintln(os.Stderr, "License required to launch Steam")
			os.Exit(1)
		}
	}
	
	// Load config and launch
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}
	
	if err := launcher.Launch(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Launch failed: %v\n", err)
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
		os.Exit(1)
	}
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
		tooltip = "Wizado Gaming Mode\\n━━━━━━━━━━━━━━━━━━━\\nLeft-click: Launch Steam\\nRight-click: Settings"
	} else {
		class = "unlicensed"
		tooltip = "Wizado Gaming Mode\\n━━━━━━━━━━━━━━━━━━━\\nLicense Required\\n$10 for 5 machines\\nwizado.app\\n━━━━━━━━━━━━━━━━━━━\\nLeft-click: Enter License\\nRight-click: Settings"
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

