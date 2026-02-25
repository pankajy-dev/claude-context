package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pankaj/claude-context/internal/common"
	"github.com/pankaj/claude-context/internal/config"
	"github.com/spf13/cobra"
)

var (
	cleanupForce   bool
	cleanupRestore bool
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Clean up orphaned symlinks and stale data",
	Long: `Clean up orphaned symlinks and stale data.

This command:
- Removes orphaned symlinks from managed projects (not in config)
- Removes orphaned context directories (not in config)
- Removes orphaned ticket directories (not in config)
- Removes orphaned global context files (not in config)
- Removes invalid symlinks (broken, wrong target, etc.)

With --restore flag:
- Adds orphaned items back to config.json instead of deleting them
- Restores orphaned context directories as managed projects (prompts for project path)
- Restores orphaned ticket directories as archived tickets
- Restores orphaned global files as disabled global contexts

Use --dry-run to preview what would be cleaned without making changes.`,
	RunE: runCleanup,
}

func init() {
	rootCmd.AddCommand(cleanupCmd)
	cleanupCmd.Flags().BoolVarP(&cleanupForce, "force", "f", false, "Skip confirmation prompts")
	cleanupCmd.Flags().BoolVarP(&cleanupRestore, "restore", "r", false, "Restore orphaned items to config.json instead of deleting")
}

type cleanupItem struct {
	itemType    string // "symlink", "context-dir", "ticket-dir", "global-file"
	path        string
	project     string // Project context name (if applicable)
	description string
}

func runCleanup(cmd *cobra.Command, args []string) error {
	// Get data directory
	dataDir := GetDataDirOrExit()

	// Load config
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	infoMsg("Scanning for orphaned symlinks and stale data...")
	fmt.Println()

	cleanupItems := []cleanupItem{}

	// 1. Scan all managed projects for orphaned symlinks
	for _, project := range cfg.ManagedProjects {
		if !common.DirExists(project.ProjectPath) {
			continue // Skip missing project directories
		}

		// Build expected symlinks for this project
		expectedSymlinks := make(map[string]bool)

		// Main context file (claude.md)
		expectedSymlinks["claude.md"] = true
		expectedSymlinks[".clauderc"] = true

		// Linked global contexts
		for _, globalName := range project.LinkedGlobals {
			gc := cfg.GetGlobalContext(globalName)
			if gc != nil {
				expectedSymlinks[filepath.Base(gc.Path)] = true
			}
		}

		// Active tickets linked to this project
		for _, ticket := range cfg.Tickets.Active {
			for _, linkedProj := range ticket.LinkedProjects {
				if linkedProj.ContextName == project.ContextName {
					// Ticket symlink format: TICKET-ID.md (e.g., CBP-123.md)
					ticketSymlink := fmt.Sprintf("%s.md", ticket.TicketID)
					expectedSymlinks[ticketSymlink] = true

					// SESSIONS symlinks (may have different names)
					// We'll be lenient and allow any SESSIONS*.md files
					// These are checked separately below
					break
				}
			}
		}

		// Scan project directory for symlinks
		entries, err := os.ReadDir(project.ProjectPath)
		if err != nil {
			warningMsg(fmt.Sprintf("Failed to read project directory %s: %v", project.ContextName, err))
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			fullPath := filepath.Join(project.ProjectPath, entry.Name())

			// Check if it's a symlink
			if !common.IsSymlink(fullPath) {
				continue
			}

			// Check if it's expected
			if expectedSymlinks[entry.Name()] {
				// Verify it's valid
				target, err := common.SymlinkTarget(fullPath)
				if err != nil || !common.FileExists(target) {
					// Broken symlink - mark for cleanup
					cleanupItems = append(cleanupItems, cleanupItem{
						itemType:    "symlink",
						path:        fullPath,
						project:     project.ContextName,
						description: fmt.Sprintf("Broken symlink: %s", entry.Name()),
					})
				}
			} else {
				// Check if it looks like a cctx-managed symlink
				isCctxManaged := false

				// Get symlink target
				target, err := common.SymlinkTarget(fullPath)

				// Check if it's a SESSIONS*.md symlink (pointing to _tickets/)
				if strings.HasPrefix(entry.Name(), "SESSIONS") && strings.HasSuffix(entry.Name(), ".md") {
					// Check if it points to a valid active ticket
					if err == nil && strings.Contains(target, "/_tickets/") {
						// Extract ticket ID from target path
						// Format: ~/.cctx/contexts/_tickets/TICKET-ID/SESSIONS.md
						parts := strings.Split(target, "/_tickets/")
						if len(parts) == 2 {
							ticketPath := strings.Split(parts[1], "/")[0]
							// Check if this ticket is still active and linked to this project
							validTicket := false
							for _, ticket := range cfg.Tickets.Active {
								if ticket.TicketID == ticketPath {
									for _, lp := range ticket.LinkedProjects {
										if lp.ContextName == project.ContextName {
											validTicket = true
											break
										}
									}
								}
								if validTicket {
									break
								}
							}

							if !validTicket {
								// Orphaned SESSIONS symlink
								isCctxManaged = true
							}
						}
					} else {
						// Broken SESSIONS symlink
						isCctxManaged = true
					}
				}

				// Check if it's a ticket symlink (matches pattern like CBP-123.md, BEE-456.md)
				// Ticket IDs usually match pattern: UPPERCASE-DIGITS.md
				if strings.HasSuffix(entry.Name(), ".md") && !strings.HasPrefix(entry.Name(), "SESSIONS") {
					// Check if target points to _tickets directory
					if err == nil && strings.Contains(target, "/_tickets/") {
						isCctxManaged = true
					}
				}

				// Check if it's a global context symlink (points to _global/)
				if err == nil && strings.Contains(target, "/_global/") {
					isCctxManaged = true
				}

				// Check if target points to data directory contexts
				if err == nil && strings.HasPrefix(target, filepath.Join(dataDir, "contexts")) {
					isCctxManaged = true
				}

				if isCctxManaged {
					// Orphaned cctx-managed symlink
					cleanupItems = append(cleanupItems, cleanupItem{
						itemType:    "symlink",
						path:        fullPath,
						project:     project.ContextName,
						description: fmt.Sprintf("Orphaned symlink: %s", entry.Name()),
					})
				}
			}
		}
	}

	// 2. Scan data directory for orphaned context directories
	contextsDir := cfgMgr.GetContextsPath()
	if common.DirExists(contextsDir) {
		entries, err := os.ReadDir(contextsDir)
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}

				// Skip special directories
				if entry.Name() == "_global" || entry.Name() == "_tickets" || entry.Name() == "_archived" {
					continue
				}

				// Check if in config
				if cfg.GetProject(entry.Name()) == nil {
					orphanedDir := filepath.Join(contextsDir, entry.Name())
					cleanupItems = append(cleanupItems, cleanupItem{
						itemType:    "context-dir",
						path:        orphanedDir,
						project:     entry.Name(),
						description: fmt.Sprintf("Orphaned context directory: %s", entry.Name()),
					})
				}
			}
		}
	}

	// 3. Scan _tickets directory for orphaned ticket directories
	ticketsDir := filepath.Join(contextsDir, "_tickets")
	if common.DirExists(ticketsDir) {
		entries, err := os.ReadDir(ticketsDir)
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}

				// Check if ticket exists in config (active or archived)
				ticketExists := false
				for _, ticket := range cfg.Tickets.Active {
					if ticket.TicketID == entry.Name() {
						ticketExists = true
						break
					}
				}
				if !ticketExists {
					for _, ticket := range cfg.Tickets.Archived {
						if ticket.TicketID == entry.Name() {
							ticketExists = true
							break
						}
					}
				}

				if !ticketExists {
					orphanedTicketDir := filepath.Join(ticketsDir, entry.Name())
					cleanupItems = append(cleanupItems, cleanupItem{
						itemType:    "ticket-dir",
						path:        orphanedTicketDir,
						project:     "",
						description: fmt.Sprintf("Orphaned ticket directory: %s", entry.Name()),
					})
				}
			}
		}
	}

	// 4. Scan _global directory for orphaned global context files
	globalDir := filepath.Join(contextsDir, "_global")
	if common.DirExists(globalDir) {
		entries, err := os.ReadDir(globalDir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}

				// Get global context name from filename (e.g., script.md -> script)
				globalName := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))

				// Check if global context exists in config
				if cfg.GetGlobalContext(globalName) == nil {
					orphanedGlobalFile := filepath.Join(globalDir, entry.Name())
					cleanupItems = append(cleanupItems, cleanupItem{
						itemType:    "global-file",
						path:        orphanedGlobalFile,
						project:     "",
						description: fmt.Sprintf("Orphaned global context file: %s", entry.Name()),
					})
				}
			}
		}
	}

	// Summary
	fmt.Println("=" + strings.Repeat("=", 60))
	fmt.Printf("Cleanup Summary\n")
	fmt.Println("=" + strings.Repeat("=", 60))
	fmt.Printf("Items to clean: %d\n", len(cleanupItems))

	if len(cleanupItems) == 0 {
		successMsg("No orphaned items found. Everything is clean!")
		return nil
	}

	// Group by type
	byType := make(map[string]int)
	for _, item := range cleanupItems {
		byType[item.itemType]++
	}

	fmt.Println("\nBreakdown:")
	if count, ok := byType["symlink"]; ok {
		fmt.Printf("  - Orphaned symlinks: %d\n", count)
	}
	if count, ok := byType["context-dir"]; ok {
		fmt.Printf("  - Orphaned context directories: %d\n", count)
	}
	if count, ok := byType["ticket-dir"]; ok {
		fmt.Printf("  - Orphaned ticket directories: %d\n", count)
	}
	if count, ok := byType["global-file"]; ok {
		fmt.Printf("  - Orphaned global files: %d\n", count)
	}
	fmt.Println()

	// Show restore option info if not already in restore mode
	if !cleanupRestore && !dryRun {
		fmt.Println("=" + strings.Repeat("=", 60))
		infoMsg("You have two options:")
		fmt.Println("  1. DELETE: Remove orphaned items from filesystem (default)")
		fmt.Println("  2. RESTORE: Add orphaned items back to config.json")
		fmt.Println()
		fmt.Println("To restore instead of delete, use: cctx cleanup --restore")
		fmt.Println("=" + strings.Repeat("=", 60))
		fmt.Println()
	}

	// Show details in verbose mode
	if verbose {
		fmt.Println("Details:")
		for i, item := range cleanupItems {
			if item.project != "" {
				fmt.Printf("%d. [%s] %s: %s\n", i+1, item.project, item.itemType, item.description)
			} else {
				fmt.Printf("%d. %s: %s\n", i+1, item.itemType, item.description)
			}
		}
		fmt.Println()
	}

	// Confirm cleanup or restore
	if dryRun {
		fmt.Println("=" + strings.Repeat("=", 60))
		if cleanupRestore {
			infoMsg("DRY RUN: Restore mode - no changes will be made")
			fmt.Println("\nWhat will happen with --restore:")
			fmt.Println("  • Context directories → Restored as managed projects (prompts for path)")
			fmt.Println("  • Ticket directories → Restored as archived tickets")
			fmt.Println("  • Global files → Restored as disabled global contexts")
			fmt.Println("  • Symlinks → Cannot be restored (manual cleanup needed)")
		} else {
			infoMsg("DRY RUN: Delete mode - no changes will be made")
			fmt.Println("\nTo restore instead of delete, use: cctx cleanup --restore --dry-run")
		}
		fmt.Println("=" + strings.Repeat("=", 60))

		if verbose {
			if cleanupRestore {
				fmt.Println("\nWould restore to config.json:")
			} else {
				fmt.Println("\nWould remove:")
			}
			for _, item := range cleanupItems {
				fmt.Printf("  - %s\n", item.path)
			}
		}
		return nil
	}

	// Show mode info before confirmation
	if cleanupRestore {
		fmt.Println("=" + strings.Repeat("=", 60))
		infoMsg("RESTORE MODE: Adding orphaned items back to config.json")
		fmt.Println("\nWhat will happen:")
		fmt.Println("  • Context directories → Restored as managed projects (prompts for path)")
		fmt.Println("  • Ticket directories → Restored as archived tickets")
		fmt.Println("  • Global files → Restored as disabled global contexts")
		fmt.Println("  • Symlinks → Cannot be restored (will be skipped)")
		fmt.Println("=" + strings.Repeat("=", 60))
		fmt.Println()
	}

	if !cleanupForce {
		var prompt string
		if cleanupRestore {
			prompt = "Restore orphaned items to config.json?"
		} else {
			prompt = "Proceed with cleanup (delete orphaned items)?"
		}
		if !common.Confirm(prompt, false) {
			infoMsg("Operation cancelled")
			return nil
		}
		fmt.Println()
	}

	// Perform cleanup or restore
	if cleanupRestore {
		return restoreOrphanedItems(cleanupItems, cfg, cfgMgr, dataDir)
	}

	return deleteOrphanedItems(cleanupItems)
}

// deleteOrphanedItems removes orphaned items from the filesystem
func deleteOrphanedItems(cleanupItems []cleanupItem) error {
	removed := 0
	failed := 0

	for _, item := range cleanupItems {
		var err error

		switch item.itemType {
		case "symlink":
			err = os.Remove(item.path)
		case "context-dir", "ticket-dir":
			err = os.RemoveAll(item.path)
		case "global-file":
			err = os.Remove(item.path)
		}

		if err != nil {
			warningMsg(fmt.Sprintf("Failed to remove %s: %v", item.description, err))
			failed++
		} else {
			removed++
			if verbose {
				successMsg(fmt.Sprintf("Removed: %s", item.description))
			}
		}
	}

	fmt.Println()
	successMsg(fmt.Sprintf("Cleanup complete: %d items removed", removed))
	if failed > 0 {
		warningMsg(fmt.Sprintf("%d items failed to remove", failed))
	}

	return nil
}

// restoreOrphanedItems adds orphaned items back to config.json
func restoreOrphanedItems(cleanupItems []cleanupItem, cfg *config.Config, cfgMgr *config.Manager, dataDir string) error {
	restored := 0
	failed := 0
	configChanged := false

	for _, item := range cleanupItems {
		var err error

		switch item.itemType {
		case "context-dir":
			// Restore as managed project
			// Need to prompt for project path
			fmt.Printf("\nRestore context directory: %s\n", item.project)
			fmt.Printf("Enter project path (absolute path to the project directory): ")
			var projectPath string
			fmt.Scanln(&projectPath)

			if projectPath == "" {
				warningMsg(fmt.Sprintf("Skipped %s: no project path provided", item.project))
				failed++
				continue
			}

			// Normalize path
			projectPath, err = common.NormalizePath(projectPath)
			if err != nil {
				warningMsg(fmt.Sprintf("Failed to restore %s: invalid path: %v", item.project, err))
				failed++
				continue
			}

			// Check if project directory exists
			if !common.DirExists(projectPath) {
				warningMsg(fmt.Sprintf("Failed to restore %s: project directory does not exist: %s", item.project, projectPath))
				failed++
				continue
			}

			// Add to config
			contextPath := filepath.Join("contexts", item.project, "claude.md")
			newProject := config.Project{
				ContextName:   item.project,
				ProjectPath:   projectPath,
				ContextPath:   contextPath,
				CreatedAt:     time.Now(),
				LastModified:  time.Now(),
				Status:        "active",
				LinkedGlobals: []string{},
			}
			cfg.AddProject(newProject)
			configChanged = true

			successMsg(fmt.Sprintf("Restored project: %s -> %s", item.project, projectPath))
			restored++

		case "ticket-dir":
			// Restore as archived ticket
			ticketID := filepath.Base(item.path)

			// Read metadata if it exists
			metadataPath := filepath.Join(item.path, "metadata.json")
			var title, notes string
			var tags []string

			if common.FileExists(metadataPath) {
				// Try to read existing metadata
				data, err := os.ReadFile(metadataPath)
				if err == nil {
					var metadata map[string]interface{}
					if json.Unmarshal(data, &metadata) == nil {
						if t, ok := metadata["title"].(string); ok {
							title = t
						}
						if n, ok := metadata["notes"].(string); ok {
							notes = n
						}
						if tgs, ok := metadata["tags"].([]interface{}); ok {
							for _, tag := range tgs {
								if tagStr, ok := tag.(string); ok {
									tags = append(tags, tagStr)
								}
							}
						}
					}
				}
			}

			if title == "" {
				title = "Restored ticket"
			}

			now := time.Now()
			archivedTicket := config.Ticket{
				TicketID:       ticketID,
				Title:          title,
				Status:         "archived",
				CreatedAt:      now,
				LastModified:   now,
				CompletedAt:    &now,
				LinkedProjects: []config.LinkedProject{},
				Tags:           tags,
				Notes:          notes,
				ArchivedPath:   filepath.Join("contexts", "_archived", ticketID),
			}

			cfg.Tickets.Archived = append(cfg.Tickets.Archived, archivedTicket)
			configChanged = true

			successMsg(fmt.Sprintf("Restored ticket: %s (archived)", ticketID))
			restored++

		case "global-file":
			// Restore as disabled global context
			globalName := strings.TrimSuffix(filepath.Base(item.path), filepath.Ext(item.path))
			globalPath := filepath.Join("contexts", "_global", filepath.Base(item.path))

			newGlobal := config.GlobalContext{
				Name:        globalName,
				Description: "Restored global context",
				Path:        globalPath,
				Enabled:     false, // Disabled by default
			}

			cfg.GlobalContexts = append(cfg.GlobalContexts, newGlobal)
			configChanged = true

			successMsg(fmt.Sprintf("Restored global context: %s (disabled)", globalName))
			restored++

		case "symlink":
			// For orphaned symlinks, we can't easily restore them
			// because we don't know what they should link to
			warningMsg(fmt.Sprintf("Cannot restore symlink: %s (delete it manually or remove from projects)", item.description))
			failed++
		}
	}

	// Save config if changes were made
	if configChanged {
		if err := cfgMgr.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		fmt.Println()
		successMsg("Config updated successfully")
	}

	fmt.Println()
	successMsg(fmt.Sprintf("Restore complete: %d items restored", restored))
	if failed > 0 {
		warningMsg(fmt.Sprintf("%d items failed to restore or were skipped", failed))
	}

	return nil
}