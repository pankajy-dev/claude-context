package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pankaj/claude-context/internal/common"
	"github.com/spf13/cobra"
)

var (
	// Global flags
	dryRun  bool
	verbose bool
	dataDir string
)

var rootCmd = &cobra.Command{
	Use:   "cctx",
	Short: "Claude Context Manager - Centralized claude.md management",
	Long: `Claude Context Manager

A centralized management system for claude.md context files across multiple
projects using symlinks. Store all context files in one repository while
keeping projects clean.

The tool automatically manages .clauderc files to ensure Claude Code includes
all relevant context files (claude.md, global.md, and ticket files).`,
	Version: "2.0.0",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global flags available to all commands
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without executing")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed information")
	rootCmd.PersistentFlags().StringVarP(&dataDir, "data-dir", "d", "", "Path to data directory (default: ~/.cctx, or CCTX_DATA_DIR env var)")
}

// Helper functions for consistent output
func successMsg(msg string) {
	fmt.Printf("✓ %s\n", msg)
}

func errorMsg(msg string) {
	fmt.Fprintf(os.Stderr, "✗ %s\n", msg)
}

func warningMsg(msg string) {
	fmt.Printf("⚠ %s\n", msg)
}

func infoMsg(msg string) {
	fmt.Printf("ℹ %s\n", msg)
}

func dryRunMsg(msg string) {
	if dryRun {
		fmt.Printf("[DRY RUN] %s\n", msg)
	}
}

// GetDataDir returns the data directory path
// Priority: 1) --data-dir flag, 2) CCTX_DATA_DIR env var, 3) ~/.cctx
// Does NOT check if directory exists or is initialized
func GetDataDir() (string, error) {
	var path string

	// 1. Check --data-dir flag
	if dataDir != "" {
		path = dataDir
	} else if envDir := os.Getenv("CCTX_DATA_DIR"); envDir != "" {
		// 2. Check CCTX_DATA_DIR environment variable
		path = envDir
	} else {
		// 3. Default to ~/.cctx
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		path = filepath.Join(homeDir, ".cctx")
	}

	// Normalize the path (expand ~, make absolute)
	absPath, err := common.NormalizePath(path)
	if err != nil {
		return "", fmt.Errorf("invalid data directory path: %w", err)
	}

	return absPath, nil
}

// GetDataDirOrExit returns the data directory path or exits with an error
// This also checks if the directory is initialized (has config.json)
func GetDataDirOrExit() string {
	dataDir, err := GetDataDir()
	if err != nil {
		errorMsg(err.Error())
		os.Exit(1)
	}

	// Check if initialized
	configPath := filepath.Join(dataDir, "config.json")
	if !common.FileExists(configPath) {
		errorMsg(fmt.Sprintf("Data directory not initialized: %s", dataDir))
		errorMsg("Run 'cctx init' to initialize")
		os.Exit(1)
	}

	return dataDir
}
