package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pankaj/claude-context/internal/common"
	"github.com/pankaj/claude-context/internal/config"
	"github.com/spf13/cobra"
)

var (
	resetKeepConfig     bool
	resetKeepClauderc   bool
	resetForce          bool
)

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset cctx to clean state",
	Long:  `Reset cctx by removing managed files at different scopes (all projects or single project).`,
}

var resetAllCmd = &cobra.Command{
	Use:   "all",
	Short: "Reset all projects and data directory",
	Long: `Reset cctx by removing all managed files and optionally the data directory.

This command will:
  1. Remove all symlinks from all managed projects
  2. Remove all concrete ticket files from all projects
  3. Remove .clauderc files from all projects
  4. Remove entire ~/.cctx directory (unless --keep-config)

With --keep-config flag:
  - Keeps config.json and templates
  - Removes all context directories (_global, _tickets, _archived, project contexts)
  - Projects remain in config but all files are removed

This is useful for:
  - Complete cleanup after broken config
  - Starting fresh while keeping project definitions
  - Removing all dangling files across all projects

Examples:
  cctx reset all                    # Complete reset (removes ~/.cctx)
  cctx reset all --keep-config      # Keep config, remove all contexts
  cctx reset all --dry-run          # Preview what would be removed`,
	RunE: runResetAll,
}

var resetProjectCmd = &cobra.Command{
	Use:   "project [project-name]",
	Short: "Remove all cctx-managed files from a project",
	Long: `Remove all cctx-managed files from a project directory.

This command removes:
  - All symlinks (claude.md, global contexts, ticket symlinks)
  - All concrete ticket files (TICKET-*.md, SESSIONS.md)
  - .clauderc file (unless --keep-clauderc)

This is useful for:
  - Cleaning up after broken config
  - Resetting a project to clean state
  - Removing dangling files from failed operations

The project remains in config.json - use 'cctx unlink' to fully remove.

Examples:
  cctx reset project                    # Clean current project
  cctx reset project my-project         # Clean specific project
  cctx reset project --keep-clauderc    # Keep .clauderc file`,
	Args: cobra.MaximumNArgs(1),
	RunE: runResetProject,
}

func init() {
	rootCmd.AddCommand(resetCmd)

	// reset all
	resetCmd.AddCommand(resetAllCmd)
	resetAllCmd.Flags().BoolVar(&resetKeepConfig, "keep-config", false, "Keep config.json and templates, only remove context files")
	resetAllCmd.Flags().BoolVarP(&resetForce, "force", "f", false, "Skip confirmation prompt")

	// reset project
	resetCmd.AddCommand(resetProjectCmd)
	resetProjectCmd.Flags().BoolVar(&resetKeepClauderc, "keep-clauderc", false, "Keep .clauderc file")
	resetProjectCmd.Flags().BoolVarP(&resetForce, "force", "f", false, "Skip confirmation prompt")
	resetProjectCmd.InheritedFlags().MarkHidden("project")
}

func runResetAll(cmd *cobra.Command, args []string) error {
	// Get data directory
	dataDir, err := GetDataDir()
	if err != nil {
		return err
	}

	// Check if data directory exists
	if !common.DirExists(dataDir) {
		successMsg("Data directory does not exist. Nothing to reset.")
		return nil
	}

	// Load config (if exists)
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	var projects []config.Project
	if err == nil {
		projects = cfg.ManagedProjects
	}

	// Calculate what will be removed
	filesToRemove := []string{}
	dirsToRemove := []string{}
	projectFilesCount := 0

	// 1. Scan all managed projects for cctx files
	for _, project := range projects {
		if !common.DirExists(project.ProjectPath) {
			continue
		}

		entries, err := os.ReadDir(project.ProjectPath)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			fullPath := filepath.Join(project.ProjectPath, entry.Name())

			// Symlinks pointing to dataDir
			if common.IsSymlink(fullPath) {
				target, err := common.SymlinkTarget(fullPath)
				if err == nil && strings.Contains(target, filepath.Join(dataDir, "contexts")) {
					filesToRemove = append(filesToRemove, fullPath)
					projectFilesCount++
				}
			}

			// Concrete ticket files
			if strings.HasSuffix(entry.Name(), ".md") {
				baseName := strings.TrimSuffix(entry.Name(), ".md")
				if strings.Contains(baseName, "-") || strings.HasPrefix(entry.Name(), "SESSIONS") {
					filesToRemove = append(filesToRemove, fullPath)
					projectFilesCount++
				}
			}

			// .clauderc
			if entry.Name() == ".clauderc" {
				filesToRemove = append(filesToRemove, fullPath)
				projectFilesCount++
			}
		}
	}

	// 2. Data directory items
	if !resetKeepConfig {
		// Remove entire data directory
		dirsToRemove = append(dirsToRemove, dataDir)
	} else {
		// Remove only context directories
		contextsDir := filepath.Join(dataDir, "contexts")
		if common.DirExists(contextsDir) {
			dirsToRemove = append(dirsToRemove, contextsDir)
		}
	}

	// Summary
	fmt.Println()
	fmt.Println("=" + strings.Repeat("=", 70))
	fmt.Println("RESET SUMMARY")
	fmt.Println("=" + strings.Repeat("=", 70))
	fmt.Println()

	if resetKeepConfig {
		fmt.Println("Mode: PARTIAL RESET (keep config)")
		fmt.Printf("  • Remove %d files from %d managed projects\n", projectFilesCount, len(projects))
		fmt.Println("  • Remove all context directories")
		fmt.Println("  • Keep config.json and templates")
	} else {
		fmt.Println("Mode: COMPLETE RESET")
		fmt.Printf("  • Remove %d files from %d managed projects\n", projectFilesCount, len(projects))
		fmt.Printf("  • Remove entire data directory: %s\n", dataDir)
	}

	fmt.Println()

	if verbose {
		if projectFilesCount > 0 {
			fmt.Println("Files to remove from projects:")
			for _, file := range filesToRemove {
				fmt.Printf("  - %s\n", file)
			}
			fmt.Println()
		}

		if len(dirsToRemove) > 0 {
			fmt.Println("Directories to remove:")
			for _, dir := range dirsToRemove {
				fmt.Printf("  - %s\n", dir)
			}
			fmt.Println()
		}
	}

	if dryRun {
		fmt.Println("=" + strings.Repeat("=", 70))
		dryRunMsg("No changes made")
		fmt.Println("=" + strings.Repeat("=", 70))
		return nil
	}

	// Confirm
	if !resetForce {
		fmt.Println("=" + strings.Repeat("=", 70))
		warningMsg("⚠️  WARNING: This action cannot be undone!")
		fmt.Println()
		if resetKeepConfig {
			warningMsg("This will remove all context files but keep config.json")
		} else {
			warningMsg("This will completely remove all cctx data and files")
		}
		fmt.Println("=" + strings.Repeat("=", 70))
		fmt.Println()

		if !common.Confirm("Are you ABSOLUTELY SURE you want to proceed?", false) {
			infoMsg("Operation cancelled")
			return nil
		}
		fmt.Println()
	}

	// Perform reset
	removed := 0
	failed := 0

	// 1. Remove files from projects
	for _, file := range filesToRemove {
		if err := os.Remove(file); err != nil {
			if verbose {
				warningMsg(fmt.Sprintf("Failed to remove %s: %v", file, err))
			}
			failed++
		} else {
			removed++
		}
	}

	// 2. Remove directories
	for _, dir := range dirsToRemove {
		if err := os.RemoveAll(dir); err != nil {
			if verbose {
				warningMsg(fmt.Sprintf("Failed to remove %s: %v", dir, err))
			}
			failed++
		} else {
			removed++
		}
	}

	// Summary
	fmt.Println()
	fmt.Println("=" + strings.Repeat("=", 70))
	successMsg("RESET COMPLETE")
	fmt.Println("=" + strings.Repeat("=", 70))
	fmt.Printf("  • Removed: %d items\n", removed)
	if failed > 0 {
		fmt.Printf("  • Failed: %d items\n", failed)
	}
	fmt.Println()

	if resetKeepConfig {
		infoMsg("Config.json and templates preserved")
		infoMsg("Run 'cctx init' to recreate context directories")
	} else {
		infoMsg("All cctx data has been removed")
		infoMsg("Run 'cctx init' to start fresh")
	}

	return nil
}

func runResetProject(cmd *cobra.Command, args []string) error {
	// Get data directory
	dataDir := GetDataDirOrExit()

	// Load config
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Determine project
	var projectName string
	if len(args) > 0 {
		projectName = args[0]
	} else {
		// Auto-detect from current directory
		projectName, err = GetProjectContext(dataDir)
		if err != nil {
			return err
		}
		if projectName == "" {
			return fmt.Errorf("no project specified and could not auto-detect from current directory")
		}
	}

	// Find project
	project := cfg.GetProject(projectName)
	if project == nil {
		return fmt.Errorf("project not found: %s", projectName)
	}

	if !common.DirExists(project.ProjectPath) {
		return fmt.Errorf("project directory does not exist: %s", project.ProjectPath)
	}

	// Scan for cctx-managed files
	filesToRemove := []string{}

	entries, err := os.ReadDir(project.ProjectPath)
	if err != nil {
		return fmt.Errorf("failed to read project directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fullPath := filepath.Join(project.ProjectPath, entry.Name())

		// Check if it's a symlink
		if common.IsSymlink(fullPath) {
			// Check if it's cctx-managed
			target, err := common.SymlinkTarget(fullPath)
			if err == nil && strings.Contains(target, filepath.Join(dataDir, "contexts")) {
				filesToRemove = append(filesToRemove, fullPath)
			}
		} else {
			// Check for concrete ticket files
			// Pattern: TICKET-123.md, CBP-123.md, etc. (uppercase + hyphen + digits)
			if strings.HasSuffix(entry.Name(), ".md") {
				// Check if it looks like a ticket file
				baseName := strings.TrimSuffix(entry.Name(), ".md")
				if strings.Contains(baseName, "-") || strings.HasPrefix(entry.Name(), "SESSIONS") {
					// Likely a ticket file
					filesToRemove = append(filesToRemove, fullPath)
				}
			}
		}

		// .clauderc file
		if entry.Name() == ".clauderc" && !resetKeepClauderc {
			filesToRemove = append(filesToRemove, fullPath)
		}
	}

	// Summary
	fmt.Println()
	infoMsg(fmt.Sprintf("Project: %s", projectName))
	infoMsg(fmt.Sprintf("Path: %s", project.ProjectPath))
	fmt.Println()

	if len(filesToRemove) == 0 {
		successMsg("No cctx-managed files found. Project is already clean!")
		return nil
	}

	fmt.Printf("Files to remove: %d\n", len(filesToRemove))
	if verbose {
		fmt.Println("\nFiles:")
		for _, file := range filesToRemove {
			fmt.Printf("  - %s\n", filepath.Base(file))
		}
	}
	fmt.Println()

	if dryRun {
		dryRunMsg("Would remove all listed files")
		return nil
	}

	// Confirm
	if !resetForce {
		warningMsg("This will permanently delete these files from the project directory")
		warningMsg("The project will remain in config.json (use 'cctx unlink' to fully remove)")
		fmt.Println()
		if !common.Confirm(fmt.Sprintf("Remove %d files from %s?", len(filesToRemove), projectName), false) {
			infoMsg("Operation cancelled")
			return nil
		}
		fmt.Println()
	}

	// Remove files
	removed := 0
	failed := 0

	for _, file := range filesToRemove {
		if err := os.Remove(file); err != nil {
			warningMsg(fmt.Sprintf("Failed to remove %s: %v", filepath.Base(file), err))
			failed++
		} else {
			removed++
			if verbose {
				successMsg(fmt.Sprintf("Removed: %s", filepath.Base(file)))
			}
		}
	}

	fmt.Println()
	successMsg(fmt.Sprintf("Cleanup complete: %d files removed", removed))
	if failed > 0 {
		warningMsg(fmt.Sprintf("%d files failed to remove", failed))
	}

	fmt.Println()
	infoMsg("Project remains in config.json")
	infoMsg("To fully remove project, use: cctx unlink " + projectName)

	return nil
}
