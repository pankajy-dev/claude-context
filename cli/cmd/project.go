package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/pankaj/claude-context/internal/clauderc"
	"github.com/pankaj/claude-context/internal/common"
	"github.com/pankaj/claude-context/internal/config"
	"github.com/spf13/cobra"
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage project-specific operations",
	Long:  `Commands for managing individual projects and their ticket associations.`,
}

var projectResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Remove all ticket symlinks from a project",
	Long: `Remove all ticket symlinks from a specific project.

This command will:
  - Remove all ticket.md symlinks from the project directory
  - Update the project's .clauderc to remove ticket references
  - NOT affect the ticket files in ~/.cctx (tickets remain active)
  - NOT unlink the project's main claude.md context

Specify the project using:
  - Flag: cctx -p api-gateway project reset
  - Env var: export CCTX_PROJECT=api-gateway && cctx project reset
  - Current directory: cd /path/to/project && cctx project reset

This is useful when:
  - You want to clean up a project's working directory
  - Starting fresh on a project without affecting tickets
  - Switching focus between different sets of tickets`,
	Args: cobra.NoArgs,
	RunE: runProjectReset,
}

var (
	resetForce bool
)

func init() {
	rootCmd.AddCommand(projectCmd)

	// project reset
	projectCmd.AddCommand(projectResetCmd)
	projectResetCmd.Flags().BoolVarP(&resetForce, "force", "f", false, "Skip confirmation prompt")
}

func runProjectReset(cmd *cobra.Command, args []string) error {
	// Get data directory
	dataDir := GetDataDirOrExit()

	// Get project context
	projectName := GetProjectContextOrExit(dataDir)

	// Load config
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Get project
	project := cfg.GetProject(projectName)
	if project == nil {
		return fmt.Errorf("project not found: %s", projectName)
	}

	// Find all tickets linked to this project
	linkedTickets := []string{}
	for _, ticket := range cfg.Tickets.Active {
		for _, lp := range ticket.LinkedProjects {
			if lp.ContextName == projectName {
				linkedTickets = append(linkedTickets, ticket.TicketID)
				break
			}
		}
	}

	if len(linkedTickets) == 0 {
		infoMsg(fmt.Sprintf("No tickets linked to project: %s", projectName))
		return nil
	}

	// Show what will be removed
	fmt.Println()
	infoMsg(fmt.Sprintf("Project: %s", projectName))
	infoMsg(fmt.Sprintf("Location: %s", project.ProjectPath))
	infoMsg(fmt.Sprintf("Linked tickets: %d", len(linkedTickets)))
	fmt.Println()
	for _, ticketID := range linkedTickets {
		ticket := cfg.GetTicket(ticketID, false)
		if ticket != nil && ticket.Title != "" {
			fmt.Printf("  • %s - %s\n", ticketID, ticket.Title)
		} else {
			fmt.Printf("  • %s\n", ticketID)
		}
	}
	fmt.Println()
	warningMsg("This will:")
	warningMsg("  - Remove all ticket symlinks from project directory")
	warningMsg("  - Update .clauderc to remove ticket references")
	warningMsg("  - Tickets remain active in ~/.cctx (not archived)")
	fmt.Println()

	// Confirmation required (unless --force or --dry-run)
	if !resetForce && !dryRun {
		if !common.Confirm("Remove all tickets from this project?", false) {
			infoMsg("Operation cancelled")
			return nil
		}
		fmt.Println()
	}

	removedCount := 0
	for _, ticketID := range linkedTickets {
		symlinkPath := filepath.Join(project.ProjectPath, ticketID+".md")

		if dryRun {
			dryRunMsg(fmt.Sprintf("Would remove symlink: %s", ticketID+".md"))
		} else {
			if common.FileExists(symlinkPath) {
				if err := common.RemoveSymlink(symlinkPath); err != nil {
					warningMsg(fmt.Sprintf("Failed to remove %s: %v", ticketID, err))
				} else {
					if verbose {
						successMsg(fmt.Sprintf("Removed: %s.md", ticketID))
					}
					removedCount++
				}
			}
		}
	}

	// Update .clauderc - remove all ticket references
	if !dryRun {
		rcMgr := clauderc.NewManager(project.ProjectPath)
		for _, ticketID := range linkedTickets {
			if err := rcMgr.RemoveFile(ticketID+".md", dryRun); err != nil {
				warningMsg(fmt.Sprintf("Failed to update .clauderc for %s: %v", ticketID, err))
			}
		}
		successMsg("Updated .clauderc")
	}

	// Git commit removed (no longer tracking in git)

	if !dryRun {
		fmt.Println()
		successMsg(fmt.Sprintf("Removed %d ticket(s) from project: %s", removedCount, projectName))
		infoMsg("Tickets remain active in ~/.cctx")
	}

	return nil
}
