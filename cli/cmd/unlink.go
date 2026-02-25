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
	keepContent bool
)

var unlinkCmd = &cobra.Command{
	Use:   "unlink <context-name>",
	Short: "Unlink a project from the context manager",
	Long: `Unlink a project from the Claude Context Manager.

This removes the symlinks and optionally deletes the context file.
Requires confirmation before proceeding.`,
	Args: cobra.ExactArgs(1),
	RunE: runUnlink,
}

func init() {
	rootCmd.AddCommand(unlinkCmd)
	unlinkCmd.Flags().BoolVar(&keepContent, "keep-content", false, "Keep context file, only remove symlink and config entry")
}

func runUnlink(cmd *cobra.Command, args []string) error {
	contextName := args[0]

	// Get data directory
	dataDir := GetDataDirOrExit()

	// Load config
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Find project
	project := cfg.GetProject(contextName)
	if project == nil {
		return fmt.Errorf("project not found: %s", contextName)
	}

	infoMsg(fmt.Sprintf("Unlinking project: %s", contextName))
	infoMsg(fmt.Sprintf("Project path: %s", project.ProjectPath))
	if !keepContent {
		warningMsg("Context file will be deleted")
	}
	fmt.Println()

	// Check if this is a primary project for any tickets
	for i := range cfg.Tickets.Active {
		if cfg.Tickets.Active[i].PrimaryContextName == contextName {
			warningMsg(fmt.Sprintf("Project %s is primary for ticket: %s", contextName, cfg.Tickets.Active[i].TicketID))
			infoMsg("Secondary project will be promoted to primary")
		}
	}

	// Confirmation required (unless dry-run)
	if !dryRun {
		if !common.Confirm("Proceed with unlink?", false) {
			infoMsg("Operation cancelled")
			return nil
		}
		fmt.Println()
	}

	// Promote secondary projects to primary for affected tickets
	if !dryRun {
		for i := range cfg.Tickets.Active {
			ticket := &cfg.Tickets.Active[i]
			if ticket.PrimaryContextName == contextName {
				if err := promoteSecondaryToPrimary(ticket, cfg, dataDir); err != nil {
					warningMsg(fmt.Sprintf("Failed to promote secondary for %s: %v", ticket.TicketID, err))
				}
			}
		}
	}

	// Copy concrete file to data dir, remove from project
	claudeMD := filepath.Join(project.ProjectPath, "claude.md")
	contextFile := filepath.Join(dataDir, project.ContextPath)

	if dryRun {
		dryRunMsg("Would copy concrete file to data dir")
		dryRunMsg("Would remove concrete file from project")
		dryRunMsg("Would remove data dir symlink")
	} else {
		// Copy concrete file to data dir (if exists and not symlink)
		if common.FileExists(claudeMD) && !common.IsSymlink(claudeMD) {
			if err := common.CopyFile(claudeMD, contextFile); err != nil {
				warningMsg(fmt.Sprintf("Failed to copy file to data dir: %v", err))
			} else {
				successMsg("Copied file to data dir")
			}
			// Remove from project
			os.Remove(claudeMD)
			successMsg("Removed file from project")
		}
		// Remove data dir symlink (if exists)
		if common.IsSymlink(contextFile) {
			common.RemoveSymlink(contextFile)
		}
	}

	// Remove global.md symlinks
	for _, gc := range cfg.GlobalContexts {
		if gc.Enabled {
			globalFile := filepath.Join(project.ProjectPath, filepath.Base(gc.Path))
			if dryRun {
				dryRunMsg(fmt.Sprintf("Would remove symlink: %s", filepath.Base(gc.Path)))
			} else {
				if common.FileExists(globalFile) {
					if err := common.RemoveSymlink(globalFile); err != nil {
						warningMsg(fmt.Sprintf("Failed to remove %s symlink: %v", filepath.Base(gc.Path), err))
					} else {
						successMsg(fmt.Sprintf("Removed %s symlink", filepath.Base(gc.Path)))
					}
				}
			}
		}
	}

	// Delete context file (unless keep-content)
	if !keepContent {
		contextDir := filepath.Join(cfgMgr.GetContextsPath(), contextName)
		if dryRun {
			dryRunMsg(fmt.Sprintf("Would delete context directory: %s", contextDir))
		} else {
			// Create backup if configured
			if cfg.Settings.BackupOnUnlink {
				backupPath := contextDir + ".backup"
				if err := os.Rename(contextDir, backupPath); err != nil {
					warningMsg(fmt.Sprintf("Failed to create backup: %v", err))
				} else {
					successMsg(fmt.Sprintf("Created backup: %s", backupPath))
				}
			} else {
				if err := os.RemoveAll(contextDir); err != nil {
					warningMsg(fmt.Sprintf("Failed to delete context directory: %v", err))
				} else {
					successMsg("Deleted context directory")
				}
			}
		}
	} else {
		infoMsg("Keeping context file (--keep-content)")
	}

	// Remove from config
	if !dryRun {
		if !cfg.RemoveProject(contextName) {
			return fmt.Errorf("failed to remove project from config")
		}
		if err := cfgMgr.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		successMsg("Updated configuration")
	} else {
		dryRunMsg("Would remove project from config")
	}

	// Git commit removed (no longer tracking in git)

	if !dryRun {
		fmt.Println()
		successMsg(fmt.Sprintf("Successfully unlinked project: %s", contextName))
	}

	return nil
}

// promoteSecondaryToPrimary promotes a secondary project to primary when primary is unlinked
func promoteSecondaryToPrimary(ticket *config.Ticket, cfg *config.Config, dataDir string) error {
	if len(ticket.LinkedProjects) <= 1 {
		// No secondary projects - copy to data dir fallback
		ticket.PrimaryContextName = ""
		return nil
	}

	// Find new primary (first secondary project)
	var newPrimary *config.LinkedProject
	for i := range ticket.LinkedProjects {
		if ticket.LinkedProjects[i].ContextName != ticket.PrimaryContextName {
			newPrimary = &ticket.LinkedProjects[i]
			break
		}
	}

	if newPrimary == nil {
		return fmt.Errorf("no secondary project found")
	}

	// Get current primary project
	oldPrimary := cfg.GetProject(ticket.PrimaryContextName)
	if oldPrimary == nil {
		return fmt.Errorf("old primary project not found")
	}

	// Copy concrete files from old primary to new primary
	oldTicketFile := filepath.Join(oldPrimary.ProjectPath, ticket.TicketID+".md")
	newTicketFile := filepath.Join(newPrimary.ProjectPath, ticket.TicketID+".md")

	if common.FileExists(oldTicketFile) && !common.IsSymlink(oldTicketFile) {
		// Remove old symlink from new primary if exists
		if common.IsSymlink(newTicketFile) {
			common.RemoveSymlink(newTicketFile)
		}

		// Copy file to new primary
		if err := common.CopyFile(oldTicketFile, newTicketFile); err != nil {
			return fmt.Errorf("failed to copy ticket file: %w", err)
		}

		// Update data dir symlink to point to new primary
		ticketDir := filepath.Join(dataDir, "contexts/_tickets", ticket.TicketID)
		dataTicketFile := filepath.Join(ticketDir, "ticket.md")
		if common.IsSymlink(dataTicketFile) {
			common.RemoveSymlink(dataTicketFile)
		}
		if err := common.CreateSymlink(newTicketFile, dataTicketFile); err != nil {
			return fmt.Errorf("failed to update data dir symlink: %w", err)
		}

		// Update remaining secondary symlinks to point to new primary
		for _, lp := range ticket.LinkedProjects {
			if lp.ContextName != newPrimary.ContextName && lp.ContextName != ticket.PrimaryContextName {
				secondaryFile := filepath.Join(lp.ProjectPath, ticket.TicketID+".md")
				if common.IsSymlink(secondaryFile) {
					common.RemoveSymlink(secondaryFile)
					common.CreateSymlink(newTicketFile, secondaryFile)
				}
			}
		}
	}

	// Update sessions file similarly
	oldSessionsFile := filepath.Join(oldPrimary.ProjectPath, "SESSIONS.md")
	newSessionsFile := filepath.Join(newPrimary.ProjectPath, "SESSIONS.md")

	if common.FileExists(oldSessionsFile) && !common.IsSymlink(oldSessionsFile) {
		if common.IsSymlink(newSessionsFile) {
			common.RemoveSymlink(newSessionsFile)
		}
		if err := common.CopyFile(oldSessionsFile, newSessionsFile); err != nil {
			warningMsg(fmt.Sprintf("Failed to copy sessions file: %v", err))
		} else {
			// Update symlinks
			ticketDir := filepath.Join(dataDir, "contexts/_tickets", ticket.TicketID)
			dataSessionsFile := filepath.Join(ticketDir, "SESSIONS.md")
			if common.IsSymlink(dataSessionsFile) {
				common.RemoveSymlink(dataSessionsFile)
			}
			common.CreateSymlink(newSessionsFile, dataSessionsFile)
		}
	}

	// Update ticket metadata
	ticket.PrimaryContextName = newPrimary.ContextName
	successMsg(fmt.Sprintf("Promoted %s to primary for ticket %s", newPrimary.ContextName, ticket.TicketID))

	return nil
}
