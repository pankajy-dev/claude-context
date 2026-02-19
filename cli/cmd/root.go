package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pankaj/claude-context/internal/common"
	"github.com/pankaj/claude-context/internal/config"
	"github.com/spf13/cobra"
)

var (
	// Global flags
	dryRun      bool
	verbose     bool
	dataDir     string
	projectFlag string
	ticketFlag  string
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
	rootCmd.PersistentFlags().StringVarP(&projectFlag, "project", "p", "", "Project context name (default: CCTX_PROJECT env var or current directory)")
	rootCmd.PersistentFlags().StringVarP(&ticketFlag, "ticket", "t", "", "Ticket ID (default: CCTX_TICKET env var)")
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

// GetProjectContext resolves the project context from various sources
// Priority: 1) --project flag, 2) CCTX_PROJECT env var, 3) current directory
// Returns the project context name or empty string if not found
func GetProjectContext(dataDir string) (string, error) {
	// 1. Check --project flag
	if projectFlag != "" {
		return projectFlag, nil
	}

	// 2. Check CCTX_PROJECT environment variable
	if envProject := os.Getenv("CCTX_PROJECT"); envProject != "" {
		return envProject, nil
	}

	// 3. Try to detect from current directory
	currentDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	// Resolve symlinks in current directory for accurate comparison
	currentDirResolved, err := filepath.EvalSymlinks(currentDir)
	if err != nil {
		// If we can't resolve, use the original path
		currentDirResolved = currentDir
	}

	// Load config to check if current directory is a managed project
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return "", fmt.Errorf("failed to load config: %w", err)
	}

	// Check if current directory matches any managed project
	for _, project := range cfg.ManagedProjects {
		// Resolve symlinks in project path as well
		projectPathResolved, err := filepath.EvalSymlinks(project.ProjectPath)
		if err != nil {
			projectPathResolved = project.ProjectPath
		}

		if projectPathResolved == currentDirResolved {
			return project.ContextName, nil
		}
	}

	return "", nil
}

// GetProjectContextOrExit resolves the project context or exits with an error
func GetProjectContextOrExit(dataDir string) string {
	projectName, err := GetProjectContext(dataDir)
	if err != nil {
		errorMsg(fmt.Sprintf("Failed to resolve project context: %v", err))
		os.Exit(1)
	}

	if projectName == "" {
		errorMsg("No project context found")
		errorMsg("Use --project/-p flag, set CCTX_PROJECT env var, or run from a managed project directory")
		os.Exit(1)
	}

	return projectName
}

// GetTicketID resolves the ticket ID from flag or environment variable
// Priority: 1) --ticket flag, 2) CCTX_TICKET env var
// Returns the ticket ID or empty string if not found
func GetTicketID() string {
	// 1. Check --ticket flag
	if ticketFlag != "" {
		return ticketFlag
	}

	// 2. Check CCTX_TICKET environment variable
	if envTicket := os.Getenv("CCTX_TICKET"); envTicket != "" {
		return envTicket
	}

	return ""
}

// GetTicketIDOrExit resolves the ticket ID or exits with an error
func GetTicketIDOrExit() string {
	ticketID := GetTicketID()
	if ticketID == "" {
		errorMsg("No ticket ID provided")
		errorMsg("Use --ticket/-t flag or set CCTX_TICKET env var")
		os.Exit(1)
	}
	return ticketID
}
