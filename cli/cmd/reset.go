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
	Short: "Remove all cctx-managed files from a project and config",
	Long: `Remove all cctx-managed files from a project directory, data directory, and config.json.

This command removes:
  - All symlinks from project directory (claude.md, global contexts, ticket symlinks)
  - All concrete ticket files from project directory (TICKET-*.md, SESSIONS.md)
  - .clauderc file from project directory (unless --keep-clauderc)
  - Project's context directory from ~/.cctx/contexts/<project-name>/
  - Ticket directories from ~/.cctx/contexts/_tickets/ if project owns concrete files
  - Project entry from config.json (if registered)
  - Project linkages from all tickets (if registered)

This completely removes the project from cctx management.

Works even if project is not registered in config.json - will clean up any
cctx-managed files found in the directory.

This is useful for:
  - Complete cleanup and deregistration of a project
  - Resetting a project to pre-cctx state
  - Removing dangling files from failed operations
  - Cleaning up directories with only ticket files

Examples:
  cctx reset project                    # Clean current project
  cctx reset project my-project         # Clean specific project
  cctx reset project --keep-clauderc    # Keep .clauderc file
  cctx reset project --dry-run -v       # Preview what will be removed`,
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

	// Collect all unique project paths (from managed projects + ticket linked projects)
	projectPaths := make(map[string]bool)
	managedProjectPaths := make(map[string]bool) // Track which paths are managed (for CLAUDE.md removal)
	for _, project := range projects {
		projectPaths[project.ProjectPath] = true
		managedProjectPaths[project.ProjectPath] = true
	}

	// Add paths from tickets (both active and archived)
	allTickets := append(cfg.Tickets.Active, cfg.Tickets.Archived...)
	for _, ticket := range allTickets {
		for _, lp := range ticket.LinkedProjects {
			projectPaths[lp.ProjectPath] = true
		}
	}

	// IMPORTANT: Also scan ticket directories to find concrete files
	// This catches tickets created in unregistered projects where linked_projects is empty
	ticketsDir := filepath.Join(dataDir, "contexts", "_tickets")
	if common.DirExists(ticketsDir) {
		ticketEntries, err := os.ReadDir(ticketsDir)
		if err == nil {
			for _, ticketEntry := range ticketEntries {
				if !ticketEntry.IsDir() {
					continue
				}
				ticketDir := filepath.Join(ticketsDir, ticketEntry.Name())

				// Check for symlinks in the ticket directory
				entries, err := os.ReadDir(ticketDir)
				if err != nil {
					continue
				}

				for _, entry := range entries {
					if entry.IsDir() {
						continue
					}

					symlinkPath := filepath.Join(ticketDir, entry.Name())
					if common.IsSymlink(symlinkPath) {
						// Follow symlink to find concrete file location
						target, err := common.SymlinkTarget(symlinkPath)
						if err == nil && common.FileExists(target) {
							// Add the directory containing the concrete file
							projectPaths[filepath.Dir(target)] = true
						}
					}
				}
			}
		}
	}

	// 1. Scan all project paths for cctx files
	for projectPath := range projectPaths {
		if !common.DirExists(projectPath) {
			continue
		}

		entries, err := os.ReadDir(projectPath)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			fullPath := filepath.Join(projectPath, entry.Name())
			fileName := entry.Name()

			// Symlinks pointing to dataDir
			if common.IsSymlink(fullPath) {
				target, err := common.SymlinkTarget(fullPath)
				if err == nil && strings.Contains(target, filepath.Join(dataDir, "contexts")) {
					filesToRemove = append(filesToRemove, fullPath)
					projectFilesCount++
				}
			}

			// Concrete files (not symlinks)
			if !common.IsSymlink(fullPath) && strings.HasSuffix(fileName, ".md") {
				// CLAUDE.md or claude.md - only remove if project is managed by cctx
				if (fileName == "CLAUDE.md" || fileName == "claude.md") && managedProjectPaths[projectPath] {
					filesToRemove = append(filesToRemove, fullPath)
					projectFilesCount++
				}

				// Ticket files (contain hyphen) or SESSIONS.md - always remove (cctx-managed)
				baseName := strings.TrimSuffix(fileName, ".md")
				if strings.Contains(baseName, "-") || strings.HasPrefix(fileName, "SESSIONS") {
					filesToRemove = append(filesToRemove, fullPath)
					projectFilesCount++
				}
			}

			// .clauderc - always cctx-managed
			if fileName == ".clauderc" {
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
		fmt.Printf("  • Remove %d files from %d project directories\n", projectFilesCount, len(projectPaths))
		fmt.Println("  • Remove all context directories")
		fmt.Println("  • Keep config.json and templates")
	} else {
		fmt.Println("Mode: COMPLETE RESET")
		fmt.Printf("  • Remove %d files from %d project directories\n", projectFilesCount, len(projectPaths))
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
	var projectPath string
	var projectInConfig bool

	if len(args) > 0 {
		projectName = args[0]
		// Check if project exists in config
		project := cfg.GetProject(projectName)
		if project != nil {
			projectPath = project.ProjectPath
			projectInConfig = true
		} else {
			// Project not in config, maybe just has orphaned files
			warningMsg(fmt.Sprintf("Project '%s' not found in config.json", projectName))
			infoMsg("Will clean up any cctx files in current directory and data directory")
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}
			projectPath = cwd
			projectInConfig = false
		}
	} else {
		// Auto-detect from current directory
		projectName, err = GetProjectContext(dataDir)
		if err != nil {
			return err
		}

		if projectName == "" {
			// Not registered, but we can still clean current directory
			warningMsg("Current directory is not a registered cctx project")
			infoMsg("Will clean up any cctx files found in current directory")
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}
			// Use directory name as project name for display
			projectName = filepath.Base(cwd)
			projectPath = cwd
			projectInConfig = false
		} else {
			// Project found in config
			project := cfg.GetProject(projectName)
			if project != nil {
				projectPath = project.ProjectPath
				projectInConfig = true
			} else {
				return fmt.Errorf("project found but missing from config (data inconsistency)")
			}
		}
	}

	if !common.DirExists(projectPath) {
		return fmt.Errorf("project directory does not exist: %s", projectPath)
	}

	// Scan for cctx-managed files
	filesToRemove := []string{}
	dirsToRemove := []string{}

	entries, err := os.ReadDir(projectPath)
	if err != nil {
		return fmt.Errorf("failed to read project directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fullPath := filepath.Join(projectPath, entry.Name())

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

	// Find data directory items to remove
	// 1. Project's context directory
	projectContextDir := filepath.Join(dataDir, "contexts", projectName)
	if common.DirExists(projectContextDir) {
		dirsToRemove = append(dirsToRemove, projectContextDir)
	}

	// 2. Ticket directories where this project is the primary context
	ticketsDir := filepath.Join(dataDir, "contexts", "_tickets")
	if common.DirExists(ticketsDir) {
		// Check both active and archived tickets
		allTickets := append(cfg.Tickets.Active, cfg.Tickets.Archived...)
		for _, ticket := range allTickets {
			// Check if this project is linked to this ticket
			isLinked := false
			for _, link := range ticket.LinkedProjects {
				if link.ContextName == projectName {
					isLinked = true
					break
				}
			}

			if isLinked {
				ticketDir := filepath.Join(ticketsDir, ticket.TicketID)
				if common.DirExists(ticketDir) {
					dirsToRemove = append(dirsToRemove, ticketDir)
				}
			}
		}
	}

	// Summary
	fmt.Println()
	infoMsg(fmt.Sprintf("Project: %s", projectName))
	infoMsg(fmt.Sprintf("Path: %s", projectPath))
	if !projectInConfig {
		warningMsg("⚠ Project not registered in config.json")
		infoMsg("Will clean up files and related data directories")
	}
	fmt.Println()

	if len(filesToRemove) == 0 && len(dirsToRemove) == 0 {
		successMsg("No cctx-managed files found. Project is already clean!")
		return nil
	}

	fmt.Printf("Files to remove: %d\n", len(filesToRemove))
	fmt.Printf("Directories to remove: %d\n", len(dirsToRemove))

	if verbose {
		if len(filesToRemove) > 0 {
			fmt.Println("\nFiles from project:")
			for _, file := range filesToRemove {
				fmt.Printf("  - %s\n", filepath.Base(file))
			}
		}
		if len(dirsToRemove) > 0 {
			fmt.Println("\nDirectories from data dir:")
			for _, dir := range dirsToRemove {
				fmt.Printf("  - %s\n", dir)
			}
		}
	}
	fmt.Println()

	if dryRun {
		dryRunMsg("Would remove all listed files and directories")
		if projectInConfig {
			dryRunMsg(fmt.Sprintf("Would remove project '%s' from config.json", projectName))
			dryRunMsg("Would remove project linkages from all tickets")
		} else {
			dryRunMsg("Would skip config.json update (project not registered)")
		}
		return nil
	}

	// Confirm
	if !resetForce {
		warningMsg("This will permanently delete:")
		warningMsg(fmt.Sprintf("  • %d files from project directory", len(filesToRemove)))
		warningMsg(fmt.Sprintf("  • %d directories from data directory (~/.cctx)", len(dirsToRemove)))
		if projectInConfig {
			warningMsg(fmt.Sprintf("  • Project entry from config.json"))
			warningMsg(fmt.Sprintf("  • All ticket linkages to this project"))
		}
		fmt.Println()
		totalItems := len(filesToRemove) + len(dirsToRemove)
		var confirmMsg string
		if projectInConfig {
			confirmMsg = fmt.Sprintf("Remove %d items and deregister %s?", totalItems, projectName)
		} else {
			confirmMsg = fmt.Sprintf("Remove %d items from %s?", totalItems, projectName)
		}
		if !common.Confirm(confirmMsg, false) {
			infoMsg("Operation cancelled")
			return nil
		}
		fmt.Println()
	}

	// Remove files and directories
	removed := 0
	failed := 0

	// 1. Remove files from project
	for _, file := range filesToRemove {
		if err := os.Remove(file); err != nil {
			warningMsg(fmt.Sprintf("Failed to remove %s: %v", filepath.Base(file), err))
			failed++
		} else {
			removed++
			if verbose {
				successMsg(fmt.Sprintf("Removed file: %s", filepath.Base(file)))
			}
		}
	}

	// 2. Remove directories from data dir
	for _, dir := range dirsToRemove {
		if err := os.RemoveAll(dir); err != nil {
			warningMsg(fmt.Sprintf("Failed to remove directory %s: %v", dir, err))
			failed++
		} else {
			removed++
			if verbose {
				successMsg(fmt.Sprintf("Removed directory: %s", dir))
			}
		}
	}

	// 3. Remove project from config.json (if it was registered)
	if projectInConfig {
		if verbose {
			infoMsg("Removing project from config.json...")
		}

		// Remove project linkages from all tickets
		for i := range cfg.Tickets.Active {
			newLinks := []config.LinkedProject{}
			for _, link := range cfg.Tickets.Active[i].LinkedProjects {
				if link.ContextName != projectName {
					newLinks = append(newLinks, link)
				}
			}
			cfg.Tickets.Active[i].LinkedProjects = newLinks
		}

		for i := range cfg.Tickets.Archived {
			newLinks := []config.LinkedProject{}
			for _, link := range cfg.Tickets.Archived[i].LinkedProjects {
				if link.ContextName != projectName {
					newLinks = append(newLinks, link)
				}
			}
			cfg.Tickets.Archived[i].LinkedProjects = newLinks
		}

		// Remove project from managed projects
		if cfg.RemoveProject(projectName) {
			if verbose {
				successMsg("Removed project from config")
			}
		} else {
			warningMsg("Project not found in config (may have been removed already)")
		}

		// Save updated config
		if err := cfgMgr.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config after cleanup: %w", err)
		}
	} else {
		if verbose {
			infoMsg("Skipping config.json update (project not registered)")
		}
	}

	fmt.Println()
	successMsg(fmt.Sprintf("Reset complete: %d items removed", removed))
	if failed > 0 {
		warningMsg(fmt.Sprintf("%d items failed to remove", failed))
	}

	fmt.Println()
	if projectInConfig {
		successMsg(fmt.Sprintf("Project '%s' has been completely removed from cctx", projectName))
		infoMsg("All files, directories, and config entries have been cleaned up")
	} else {
		successMsg(fmt.Sprintf("Cleaned up cctx files from '%s'", projectName))
		infoMsg("All files and directories have been removed")
		infoMsg("Project was not registered in config.json (no config changes made)")
	}

	return nil
}
