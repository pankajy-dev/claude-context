package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/pankaj/claude-context/internal/clauderc"
	"github.com/pankaj/claude-context/internal/common"
	"github.com/pankaj/claude-context/internal/config"
	"github.com/spf13/cobra"
)

var ticketCmd = &cobra.Command{
	Use:   "ticket",
	Short: "Manage ticket workspaces",
	Long:  `Create and manage ticket workspaces for tracking work across multiple projects.`,
}

// ticket create subcommand
var ticketCreateCmd = &cobra.Command{
	Use:   "create [<ticket-id>]",
	Short: "Create a new ticket workspace",
	Long: `Create a new ticket workspace for tracking work across multiple projects.

Automatically creates a symlink in the current directory and updates .clauderc
to include the ticket file.

If ticket-id is not provided, it will be auto-detected from the current git branch
(only if the branch is not main/master). If a ticket with that name already exists,
a suffix like -1, -2, etc. will be appended automatically.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTicketCreate,
}

var (
	ticketTitle   string
	ticketTags    string
	ticketNotes   string
	ticketCommits string
	ticketPRs     string
	ticketReason  string
	ticketStatus  string
	ticketAll     bool
	ticketForce   bool
)

func init() {
	rootCmd.AddCommand(ticketCmd)

	// ticket create
	ticketCmd.AddCommand(ticketCreateCmd)
	ticketCreateCmd.Flags().StringVar(&ticketTitle, "title", "", "Human-readable title for the ticket")
	ticketCreateCmd.Flags().StringVar(&ticketTags, "tags", "", "Comma-separated tags")
	ticketCreateCmd.Flags().StringVar(&ticketNotes, "notes", "", "Additional notes or context")

	// ticket link
	ticketCmd.AddCommand(ticketLinkCmd)

	// ticket unlink
	ticketCmd.AddCommand(ticketUnlinkCmd)
	ticketUnlinkCmd.Flags().BoolVar(&ticketAll, "all", false, "Unlink from all projects")

	// ticket list
	ticketCmd.AddCommand(ticketListCmd)
	ticketListCmd.Flags().StringVar(&ticketStatus, "status", "active", "Filter by status (active|completed|abandoned|archived|all)")

	// ticket show
	ticketCmd.AddCommand(ticketShowCmd)

	// ticket complete
	ticketCmd.AddCommand(ticketCompleteCmd)
	ticketCompleteCmd.Flags().StringVar(&ticketCommits, "commits", "", "Comma-separated commit hashes")
	ticketCompleteCmd.Flags().StringVar(&ticketPRs, "prs", "", "Comma-separated PR numbers")

	// ticket abandon
	ticketCmd.AddCommand(ticketAbandonCmd)
	ticketAbandonCmd.Flags().StringVar(&ticketReason, "reason", "", "Reason for abandoning")

	// ticket archive
	ticketCmd.AddCommand(ticketArchiveCmd)

	// ticket edit
	ticketCmd.AddCommand(ticketEditCmd)
	ticketEditCmd.Flags().StringVar(&ticketTitle, "title", "", "Update title")
	ticketEditCmd.Flags().StringVar(&ticketTags, "tags", "", "Update tags")
	ticketEditCmd.Flags().StringVar(&ticketNotes, "notes", "", "Update notes")

	// ticket delete
	ticketCmd.AddCommand(ticketDeleteCmd)
	ticketDeleteCmd.Flags().BoolVarP(&ticketForce, "force", "f", false, "Skip confirmation prompt")

	// ticket archive-all
	ticketCmd.AddCommand(ticketArchiveAllCmd)
	ticketArchiveAllCmd.Flags().BoolVarP(&ticketForce, "force", "f", false, "Skip confirmation prompt")
}

func runTicketCreate(cmd *cobra.Command, args []string) error {
	var ticketID string

	// Get data directory
	dataDir := GetDataDirOrExit()

	// Load config
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Determine ticket ID: from args or auto-detect
	if len(args) > 0 {
		// Explicit ticket ID provided
		ticketID = args[0]
		if cfg.GetTicket(ticketID, true) != nil {
			return fmt.Errorf("ticket already exists: %s", ticketID)
		}
	} else {
		// Auto-detect from branch
		branch := common.GetGitBranch()
		if branch == "" || branch == "main" || branch == "master" {
			return fmt.Errorf("cannot auto-detect ticket ID: not on a feature branch (current: %s)", branch)
		}

		// Find unique ticket ID by appending suffix if needed
		ticketID = branch
		originalID := ticketID
		suffix := 0

		for cfg.GetTicket(ticketID, true) != nil {
			suffix++
			ticketID = fmt.Sprintf("%s-%d", originalID, suffix)
		}

		if suffix > 0 {
			infoMsg(fmt.Sprintf("Ticket %s already exists, using: %s", originalID, ticketID))
		} else {
			infoMsg(fmt.Sprintf("Auto-detected ticket ID from branch: %s", ticketID))
		}
	}

	// Create ticket directory
	ticketDir := filepath.Join(cfgMgr.GetContextsPath(), "_tickets", ticketID)
	ticketFile := filepath.Join(ticketDir, "ticket.md")

	if dryRun {
		dryRunMsg(fmt.Sprintf("Would create ticket directory: %s", ticketDir))
		dryRunMsg(fmt.Sprintf("Would create ticket file: %s", ticketFile))
	} else {
		if err := common.EnsureDir(ticketDir); err != nil {
			return fmt.Errorf("failed to create ticket directory: %w", err)
		}
		successMsg(fmt.Sprintf("Created ticket directory"))

		// Create ticket.md from template
		content := fmt.Sprintf(`# Ticket: %s

## Jira Summary, description, and acceptance criteria etc.

## Notes

## For all the interaction with the claude, write the summary of interaction in [SESSIONS.md](SESSIONS.md) - This is a symlink to the SESSIONS.md file.

`, ticketID)
		if err := os.WriteFile(ticketFile, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to create ticket file: %w", err)
		}
		successMsg("Created ticket.md")

		// Create SESSIONS.md from template
		sessionsFile := filepath.Join(ticketDir, "SESSIONS.md")
		sessionsTemplate := filepath.Join(dataDir, "templates", "sessions.md")

		// Read template if it exists, otherwise use default content
		var sessionsContent []byte
		if common.FileExists(sessionsTemplate) {
			sessionsContent, err = os.ReadFile(sessionsTemplate)
			if err != nil {
				warningMsg(fmt.Sprintf("Failed to read sessions template: %v", err))
				sessionsContent = []byte(getDefaultSessionsContent())
			}
		} else {
			sessionsContent = []byte(getDefaultSessionsContent())
		}

		if err := os.WriteFile(sessionsFile, sessionsContent, 0644); err != nil {
			warningMsg(fmt.Sprintf("Failed to create SESSIONS.md: %v", err))
		} else {
			successMsg("Created SESSIONS.md")
		}
	}

	// Parse tags
	var tags []string
	if ticketTags != "" {
		tags = splitAndTrim(ticketTags, ",")
	}

	// Try to detect current project for auto-linking
	var linkedProjects []config.LinkedProject
	projectName, _ := GetProjectContext(dataDir)
	if projectName != "" {
		// Auto-link to current project
		project := cfg.GetProject(projectName)
		if project != nil {
			linkedProjects = append(linkedProjects, config.LinkedProject{
				ContextName: projectName,
				ProjectPath: project.ProjectPath,
			})
			if !dryRun {
				infoMsg(fmt.Sprintf("Auto-detected project: %s", projectName))
			}
		}
	}

	// Create ticket metadata
	ticket := config.Ticket{
		TicketID:       ticketID,
		Title:          ticketTitle,
		Status:         "active",
		CreatedAt:      time.Now(),
		LastModified:   time.Now(),
		LinkedProjects: linkedProjects,
		Tags:           tags,
		Notes:          ticketNotes,
	}

	// Add to config
	if !dryRun {
		cfg.Tickets.Active = append(cfg.Tickets.Active, ticket)
		if err := cfgMgr.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		successMsg("Updated configuration")
	}

	// Create symlink in current directory
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	symlinkName := ticketID + ".md"
	symlinkPath := filepath.Join(currentDir, symlinkName)
	sessionsSymlinkName := "SESSIONS.md"
	sessionsSymlinkPath := filepath.Join(currentDir, sessionsSymlinkName)
	sessionsFile := filepath.Join(ticketDir, "SESSIONS.md")

	if dryRun {
		dryRunMsg(fmt.Sprintf("Would create symlink: %s", symlinkPath))
		dryRunMsg(fmt.Sprintf("Would create symlink: %s (for human reference only)", sessionsSymlinkPath))
		dryRunMsg(fmt.Sprintf("Would add '%s' to .clauderc", symlinkName))
	} else {
		// Create ticket.md symlink
		if common.FileExists(symlinkPath) {
			warningMsg(fmt.Sprintf("File already exists: %s", symlinkPath))
		} else {
			if err := common.CreateSymlink(ticketFile, symlinkPath); err != nil {
				return fmt.Errorf("failed to create symlink: %w", err)
			}
			successMsg(fmt.Sprintf("Created symlink: %s", symlinkName))
		}

		// Create SESSIONS.md symlink
		if common.FileExists(sessionsSymlinkPath) {
			warningMsg(fmt.Sprintf("File already exists: %s", sessionsSymlinkPath))
		} else {
			if err := common.CreateSymlink(sessionsFile, sessionsSymlinkPath); err != nil {
				warningMsg(fmt.Sprintf("Failed to create SESSIONS.md symlink: %v", err))
			} else {
				successMsg(fmt.Sprintf("Created symlink: %s", sessionsSymlinkName))
			}
		}

		// Update .clauderc with ticket.md only (not SESSIONS.md - that's for human reference)
		rcMgr := clauderc.NewManager(currentDir)
		if err := rcMgr.AddFile(symlinkName, dryRun); err != nil {
			warningMsg(fmt.Sprintf("Failed to add %s to .clauderc: %v", symlinkName, err))
		} else {
			successMsg(fmt.Sprintf("Added %s to .clauderc", symlinkName))
		}
		// Note: SESSIONS.md is NOT added to .clauderc - it's for human reference only
	}

	// Git commit removed (no longer tracking in git)

	if !dryRun {
		fmt.Println()
		successMsg(fmt.Sprintf("Successfully created ticket: %s", ticketID))
		if ticketTitle != "" {
			infoMsg(fmt.Sprintf("Title: %s", ticketTitle))
		}
		infoMsg(fmt.Sprintf("Location: %s", ticketDir))
		infoMsg(fmt.Sprintf("Symlink: %s", symlinkPath))
		if len(ticket.LinkedProjects) > 0 {
			infoMsg(fmt.Sprintf("Auto-linked to project: %s", ticket.LinkedProjects[0].ContextName))
		}
		fmt.Println()
		infoMsg("Next steps:")
		infoMsg(fmt.Sprintf("  1. Edit ticket context: vim %s", symlinkName))
		if len(ticket.LinkedProjects) == 0 {
			infoMsg(fmt.Sprintf("  2. Link to a project: cctx ticket link %s <project>", ticketID))
		} else {
			infoMsg(fmt.Sprintf("  2. (Optional) Link to other projects: cctx -t %s ticket link <project>", ticketID))
		}
	}

	return nil
}

// ticket link subcommand
var ticketLinkCmd = &cobra.Command{
	Use:   "link [<project>...]",
	Short: "Link ticket to one or more projects",
	Long: `Link a ticket to one or more projects.

Creates symlinks in project directories and updates .clauderc automatically.

Specify ticket and projects using:
  - Ticket flag + project args: cctx -t TICKET-123 ticket link api-gateway frontend
  - Ticket flag + project flag: cctx -t TICKET-123 -p api-gateway ticket link
  - Both env vars: export CCTX_TICKET=TICKET-123 CCTX_PROJECT=api-gateway && cctx ticket link`,
	Args: cobra.ArbitraryArgs,
	RunE: runTicketLink,
}

func runTicketLink(cmd *cobra.Command, args []string) error {
	// Get data directory
	dataDir := GetDataDirOrExit()

	// Get ticket ID from flag or env var
	ticketID := GetTicketIDOrExit()

	// Determine project names - args are now always project names
	projectNames := []string{}

	if len(args) > 0 {
		// Projects specified as arguments
		projectNames = args
	} else {
		// Try to get from --project flag or env var
		projectName, err := GetProjectContext(dataDir)
		if err != nil {
			return err
		}
		if projectName == "" {
			return fmt.Errorf("no projects specified. Use --project/-p flag, set CCTX_PROJECT, or provide project names as arguments")
		}
		projectNames = []string{projectName}
	}

	// Load config
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Find ticket
	ticket := cfg.GetTicket(ticketID, false)
	if ticket == nil {
		return fmt.Errorf("ticket not found: %s", ticketID)
	}

	if ticket.Status != "active" {
		return fmt.Errorf("ticket is not active (status: %s)", ticket.Status)
	}

	ticketFile := filepath.Join(cfgMgr.GetContextsPath(), "_tickets", ticketID, "ticket.md")
	if !common.FileExists(ticketFile) {
		return fmt.Errorf("ticket file does not exist: %s", ticketFile)
	}

	fmt.Println()
	infoMsg(fmt.Sprintf("Linking ticket %s to %d project(s)", ticketID, len(projectNames)))
	fmt.Println()

	successCount := 0
	for _, projectName := range projectNames {
		fmt.Printf("Processing: %s\n", projectName)

		project := cfg.GetProject(projectName)
		if project == nil {
			errorMsg(fmt.Sprintf("Project not found: %s", projectName))
			continue
		}

		// Check if already linked
		alreadyLinked := false
		for _, lp := range ticket.LinkedProjects {
			if lp.ContextName == projectName {
				alreadyLinked = true
				break
			}
		}

		if alreadyLinked {
			infoMsg("Already linked")
			continue
		}

		symlinkPath := filepath.Join(project.ProjectPath, ticketID+".md")

		if dryRun {
			dryRunMsg(fmt.Sprintf("Would create symlink: %s", symlinkPath))
			dryRunMsg(fmt.Sprintf("Would add to .clauderc"))
		} else {
			// Create symlink
			if err := common.CreateSymlink(ticketFile, symlinkPath); err != nil {
				errorMsg(fmt.Sprintf("Failed to create symlink: %v", err))
				continue
			}
			successMsg("Created symlink")

			// Update .clauderc
			rcMgr := clauderc.NewManager(project.ProjectPath)
			if err := rcMgr.AddFile(ticketID+".md", dryRun); err != nil {
				warningMsg(fmt.Sprintf("Failed to update .clauderc: %v", err))
			} else {
				successMsg("Updated .clauderc")
			}

			// Add to ticket's linked projects
			ticket.LinkedProjects = append(ticket.LinkedProjects, config.LinkedProject{
				ContextName: projectName,
				ProjectPath: project.ProjectPath,
			})
			ticket.LastModified = time.Now()

			successCount++
		}
		fmt.Println()
	}

	// Save config
	if !dryRun && successCount > 0 {
		if err := cfgMgr.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		successMsg("Updated configuration")
	}

	// Git commit removed (no longer tracking in git)

	if !dryRun {
		fmt.Println()
		successMsg(fmt.Sprintf("Linked to %d project(s)", successCount))
	}

	return nil
}

// ticket unlink subcommand
var ticketUnlinkCmd = &cobra.Command{
	Use:   "unlink [<project>...]",
	Short: "Unlink ticket from project(s)",
	Long: `Unlink a ticket from one or more projects.

Removes symlinks and updates .clauderc automatically.

Specify ticket and projects using:
  - Ticket flag + project args: cctx -t TICKET-123 ticket unlink api-gateway frontend
  - Ticket flag + project flag: cctx -t TICKET-123 -p api-gateway ticket unlink
  - Both env vars: export CCTX_TICKET=TICKET-123 CCTX_PROJECT=api-gateway && cctx ticket unlink
  - Use --all to unlink from all projects: cctx -t TICKET-123 ticket unlink --all`,
	Args: cobra.ArbitraryArgs,
	RunE: runTicketUnlink,
}

func runTicketUnlink(cmd *cobra.Command, args []string) error {
	// Get data directory
	dataDir := GetDataDirOrExit()

	// Get ticket ID from flag or env var
	ticketID := GetTicketIDOrExit()

	// Load config
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Find ticket
	ticket := cfg.GetTicket(ticketID, false)
	if ticket == nil {
		return fmt.Errorf("ticket not found: %s", ticketID)
	}

	// Determine project names - args are now always project names
	projectNames := []string{}

	if ticketAll {
		// --all flag: unlink from all linked projects
		for _, lp := range ticket.LinkedProjects {
			projectNames = append(projectNames, lp.ContextName)
		}
	} else if len(args) > 0 {
		// Projects specified as arguments
		projectNames = args
	} else {
		// Try to get from --project flag or env var
		projectName, err := GetProjectContext(dataDir)
		if err != nil {
			return err
		}
		if projectName == "" {
			return fmt.Errorf("no projects specified. Use --project/-p flag, set CCTX_PROJECT, provide project names as arguments, or use --all")
		}
		projectNames = []string{projectName}
	}

	if len(projectNames) == 0 {
		infoMsg("No projects to unlink")
		return nil
	}

	fmt.Println()
	infoMsg(fmt.Sprintf("Unlinking ticket %s from %d project(s)", ticketID, len(projectNames)))
	fmt.Println()

	successCount := 0
	for _, projectName := range projectNames {
		fmt.Printf("Processing: %s\n", projectName)

		project := cfg.GetProject(projectName)
		if project == nil {
			warningMsg(fmt.Sprintf("Project not found: %s", projectName))
			continue
		}

		symlinkPath := filepath.Join(project.ProjectPath, ticketID+".md")

		if dryRun {
			dryRunMsg(fmt.Sprintf("Would remove symlink: %s", symlinkPath))
			dryRunMsg(fmt.Sprintf("Would remove from .clauderc"))
		} else {
			// Remove symlink
			if common.FileExists(symlinkPath) {
				if err := common.RemoveSymlink(symlinkPath); err != nil {
					warningMsg(fmt.Sprintf("Failed to remove symlink: %v", err))
				} else {
					successMsg("Removed symlink")
				}
			}

			// Update .clauderc
			rcMgr := clauderc.NewManager(project.ProjectPath)
			if err := rcMgr.RemoveFile(ticketID+".md", dryRun); err != nil {
				warningMsg(fmt.Sprintf("Failed to update .clauderc: %v", err))
			} else {
				successMsg("Updated .clauderc")
			}

			// Remove from ticket's linked projects
			newLinked := []config.LinkedProject{}
			for _, lp := range ticket.LinkedProjects {
				if lp.ContextName != projectName {
					newLinked = append(newLinked, lp)
				}
			}
			ticket.LinkedProjects = newLinked
			ticket.LastModified = time.Now()

			successCount++
		}
		fmt.Println()
	}

	// Save config
	if !dryRun && successCount > 0 {
		if err := cfgMgr.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		successMsg("Updated configuration")
	}

	// Git commit removed (no longer tracking in git)

	if !dryRun {
		fmt.Println()
		successMsg(fmt.Sprintf("Unlinked from %d project(s)", successCount))
	}

	return nil
}

// ticket list subcommand
var ticketListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tickets",
	Long:  `List all tickets with their status and linked projects.`,
	RunE:  runTicketList,
}

func runTicketList(cmd *cobra.Command, args []string) error {
	// Get data directory
	dataDir := GetDataDirOrExit()

	// Load config
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if we should filter by project
	var projectFilter string
	if projectFlag != "" || os.Getenv("CCTX_PROJECT") != "" {
		projectFilter, err = GetProjectContext(dataDir)
		if err != nil {
			return err
		}

		// Verify project exists
		if cfg.GetProject(projectFilter) == nil {
			return fmt.Errorf("project not found: %s", projectFilter)
		}
	}

	var tickets []config.Ticket

	// Filter by status
	switch ticketStatus {
	case "active":
		tickets = cfg.Tickets.Active
	case "archived":
		tickets = cfg.Tickets.Archived
	case "completed":
		for _, t := range cfg.Tickets.Active {
			if t.Status == "completed" {
				tickets = append(tickets, t)
			}
		}
		for _, t := range cfg.Tickets.Archived {
			if t.Status == "completed" {
				tickets = append(tickets, t)
			}
		}
	case "abandoned":
		for _, t := range cfg.Tickets.Active {
			if t.Status == "abandoned" {
				tickets = append(tickets, t)
			}
		}
		for _, t := range cfg.Tickets.Archived {
			if t.Status == "abandoned" {
				tickets = append(tickets, t)
			}
		}
	case "all":
		tickets = append(tickets, cfg.Tickets.Active...)
		tickets = append(tickets, cfg.Tickets.Archived...)
	default:
		return fmt.Errorf("invalid status filter: %s", ticketStatus)
	}

	// Filter by project if specified
	if projectFilter != "" {
		filteredTickets := []config.Ticket{}
		for _, ticket := range tickets {
			for _, lp := range ticket.LinkedProjects {
				if lp.ContextName == projectFilter {
					filteredTickets = append(filteredTickets, ticket)
					break
				}
			}
		}
		tickets = filteredTickets
	}

	if len(tickets) == 0 {
		if projectFilter != "" {
			infoMsg(fmt.Sprintf("No tickets found for project '%s' (status: %s)", projectFilter, ticketStatus))
		} else {
			infoMsg(fmt.Sprintf("No tickets found (status: %s)", ticketStatus))
		}
		return nil
	}

	// Show filter info
	if projectFilter != "" {
		fmt.Println()
		infoMsg(fmt.Sprintf("Showing tickets for project: %s", projectFilter))
		fmt.Println()
	}

	// Print table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	if verbose {
		fmt.Fprintln(w, "TICKET ID\tSTATUS\tTITLE\tPROJECTS\tCREATED\t")
	} else {
		fmt.Fprintln(w, "TICKET ID\tSTATUS\tTITLE\tPROJECTS\t")
	}

	for _, ticket := range tickets {
		projectCount := len(ticket.LinkedProjects)
		title := ticket.Title
		if title == "" {
			title = "-"
		}

		if verbose {
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t\n",
				ticket.TicketID,
				ticket.Status,
				title,
				projectCount,
				ticket.CreatedAt.Format("2006-01-02"),
			)
		} else {
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t\n",
				ticket.TicketID,
				ticket.Status,
				title,
				projectCount,
			)
		}
	}

	w.Flush()

	fmt.Printf("\nTotal: %d tickets\n", len(tickets))

	return nil
}

// ticket show subcommand
var ticketShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show detailed ticket information",
	Long: `Show detailed information about a specific ticket.

Specify the ticket using:
  - Flag: cctx -t TICKET-123 ticket show
  - Env var: export CCTX_TICKET=TICKET-123 && cctx ticket show`,
	Args: cobra.NoArgs,
	RunE: runTicketShow,
}

func runTicketShow(cmd *cobra.Command, args []string) error {
	ticketID := GetTicketIDOrExit()

	// Get data directory
	dataDir := GetDataDirOrExit()

	// Load config
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Find ticket
	ticket := cfg.GetTicket(ticketID, true)
	if ticket == nil {
		return fmt.Errorf("ticket not found: %s", ticketID)
	}

	// Display ticket info
	fmt.Println()
	fmt.Printf("Ticket: %s\n", ticket.TicketID)
	fmt.Println(string(make([]byte, 60)))

	if ticket.Title != "" {
		fmt.Printf("Title:        %s\n", ticket.Title)
	}
	fmt.Printf("Status:       %s\n", ticket.Status)
	fmt.Printf("Created:      %s\n", ticket.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Last Modified: %s\n", ticket.LastModified.Format("2006-01-02 15:04:05"))

	if ticket.CompletedAt != nil {
		fmt.Printf("Completed:    %s\n", ticket.CompletedAt.Format("2006-01-02 15:04:05"))
	}
	if ticket.AbandonedAt != nil {
		fmt.Printf("Abandoned:    %s\n", ticket.AbandonedAt.Format("2006-01-02 15:04:05"))
	}

	if len(ticket.Tags) > 0 {
		fmt.Printf("Tags:         %s\n", ticket.Tags)
	}

	if ticket.Notes != "" {
		fmt.Printf("Notes:        %s\n", ticket.Notes)
	}

	fmt.Println()
	fmt.Printf("Linked Projects: %d\n", len(ticket.LinkedProjects))
	for _, lp := range ticket.LinkedProjects {
		fmt.Printf("  - %s (%s)\n", lp.ContextName, lp.ProjectPath)
	}

	if len(ticket.Commits) > 0 {
		fmt.Println()
		fmt.Printf("Commits:\n")
		for _, commit := range ticket.Commits {
			fmt.Printf("  - %s\n", commit)
		}
	}

	if len(ticket.PullRequests) > 0 {
		fmt.Println()
		fmt.Printf("Pull Requests:\n")
		for _, pr := range ticket.PullRequests {
			fmt.Printf("  - %s\n", pr)
		}
	}

	if ticket.ArchivedPath != "" {
		fmt.Println()
		fmt.Printf("Archived Path: %s\n", ticket.ArchivedPath)
	}

	fmt.Println()

	return nil
}

// ticket complete subcommand
var ticketCompleteCmd = &cobra.Command{
	Use:   "complete",
	Short: "Mark ticket as completed and auto-archive",
	Long: `Mark a ticket as completed and automatically archive it.

This command will:
  - Mark ticket as completed
  - Auto-detect current git branch and commit (if in a git repo)
  - Remove ticket symlinks from all linked projects
  - Archive the ticket to ~/.cctx/contexts/_archived/
  - Update all .clauderc files

Git info is auto-detected from your current directory:
  - Branch name: stored in ticket notes
  - Latest commit: stored in ticket commits
  - Override with --commits flag if needed

Specify the ticket using:
  - Flag: cctx -t TICKET-123 ticket complete
  - Env var: export CCTX_TICKET=TICKET-123 && cctx ticket complete

Examples:
  # Auto-detect everything
  cd /path/to/project && cctx -t TICKET-123 ticket complete

  # Manual override
  cctx -t TICKET-123 ticket complete --commits "abc123,def456" --prs "42,43"`,
	Args: cobra.NoArgs,
	RunE: runTicketComplete,
}

func runTicketComplete(cmd *cobra.Command, args []string) error {
	ticketID := GetTicketIDOrExit()

	// Get data directory
	dataDir := GetDataDirOrExit()

	// Load config
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Find ticket
	ticket := cfg.GetTicket(ticketID, false)
	if ticket == nil {
		return fmt.Errorf("ticket not found: %s", ticketID)
	}

	if ticket.Status == "completed" {
		return fmt.Errorf("ticket is already completed")
	}

	// Update ticket
	completedAt := time.Now()
	ticket.Status = "completed"
	ticket.CompletedAt = &completedAt
	ticket.LastModified = time.Now()

	// Auto-detect git info if not provided
	if ticketCommits == "" {
		// Try to get commit from current directory
		commit := common.GetGitCommitShort()
		if commit != "" {
			ticket.Commits = []string{commit}
			infoMsg(fmt.Sprintf("Auto-detected commit: %s", commit))
		}
	} else {
		// Parse provided commits
		commits := []string{}
		for _, c := range splitAndTrim(ticketCommits, ",") {
			commits = append(commits, c)
		}
		ticket.Commits = commits
	}

	// Auto-detect branch name
	branch := common.GetGitBranch()
	if branch != "" {
		infoMsg(fmt.Sprintf("Auto-detected branch: %s", branch))
		// Store branch in notes if not already present
		if ticket.Notes == "" {
			ticket.Notes = fmt.Sprintf("Branch: %s", branch)
		} else if !strings.Contains(ticket.Notes, branch) {
			ticket.Notes = fmt.Sprintf("%s\nBranch: %s", ticket.Notes, branch)
		}
	}

	// Parse PRs
	if ticketPRs != "" {
		prs := []string{}
		for _, p := range splitAndTrim(ticketPRs, ",") {
			prs = append(prs, p)
		}
		ticket.PullRequests = prs
	}

	if dryRun {
		dryRunMsg(fmt.Sprintf("Would mark ticket %s as completed", ticketID))
		dryRunMsg("Would auto-archive and remove symlinks")
		return nil
	}

	// Save config first (before archiving)
	if err := cfgMgr.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println()
	successMsg(fmt.Sprintf("Ticket %s marked as completed", ticketID))

	// Auto-archive: Remove symlinks from all linked projects
	fmt.Println()
	infoMsg("Auto-archiving ticket (removing from projects)...")

	for _, lp := range ticket.LinkedProjects {
		project := cfg.GetProject(lp.ContextName)
		if project == nil {
			continue
		}

		symlinkPath := filepath.Join(project.ProjectPath, ticketID+".md")

		if common.FileExists(symlinkPath) {
			if err := common.RemoveSymlink(symlinkPath); err != nil {
				warningMsg(fmt.Sprintf("Failed to remove symlink from %s: %v", lp.ContextName, err))
			} else {
				successMsg(fmt.Sprintf("Removed symlink from %s", lp.ContextName))
			}
		}

		// Update .clauderc
		rcMgr := clauderc.NewManager(project.ProjectPath)
		if err := rcMgr.RemoveFile(ticketID+".md", dryRun); err != nil {
			warningMsg(fmt.Sprintf("Failed to update .clauderc in %s: %v", lp.ContextName, err))
		}
	}

	// Move ticket to archived
	ticketDir := filepath.Join(cfgMgr.GetContextsPath(), "_tickets", ticketID)
	archivedDir := filepath.Join(cfgMgr.GetContextsPath(), "_archived", fmt.Sprintf("%s_%s", time.Now().Format("2006-01-02"), ticketID))

	if err := common.EnsureDir(filepath.Dir(archivedDir)); err != nil {
		return fmt.Errorf("failed to create archived directory: %w", err)
	}

	if err := os.Rename(ticketDir, archivedDir); err != nil {
		return fmt.Errorf("failed to move ticket to archived: %w", err)
	}
	successMsg("Moved ticket to archived")

	// Update config: move from active to archived
	ticket.ArchivedPath = fmt.Sprintf("contexts/_archived/%s_%s", time.Now().Format("2006-01-02"), ticketID)

	// Remove from active tickets
	newActive := []config.Ticket{}
	for _, t := range cfg.Tickets.Active {
		if t.TicketID != ticketID {
			newActive = append(newActive, t)
		}
	}
	cfg.Tickets.Active = newActive

	// Add to archived tickets
	cfg.Tickets.Archived = append(cfg.Tickets.Archived, *ticket)

	if err := cfgMgr.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	successMsg("Updated configuration")

	fmt.Println()
	successMsg(fmt.Sprintf("Successfully completed and archived: %s", ticketID))
	infoMsg(fmt.Sprintf("Archived location: %s", archivedDir))
	if len(ticket.Commits) > 0 {
		infoMsg(fmt.Sprintf("Commits: %s", strings.Join(ticket.Commits, ", ")))
	}
	if branch != "" {
		infoMsg(fmt.Sprintf("Branch: %s", branch))
	}

	return nil
}

// ticket abandon subcommand
var ticketAbandonCmd = &cobra.Command{
	Use:   "abandon",
	Short: "Mark ticket as abandoned",
	Long: `Mark a ticket as abandoned with optional reason.

Specify the ticket using:
  - Flag: cctx -t TICKET-123 ticket abandon
  - Env var: export CCTX_TICKET=TICKET-123 && cctx ticket abandon`,
	Args: cobra.NoArgs,
	RunE: runTicketAbandon,
}

func runTicketAbandon(cmd *cobra.Command, args []string) error {
	ticketID := GetTicketIDOrExit()

	// Get data directory
	dataDir := GetDataDirOrExit()

	// Load config
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Find ticket
	ticket := cfg.GetTicket(ticketID, false)
	if ticket == nil {
		return fmt.Errorf("ticket not found: %s", ticketID)
	}

	if ticket.Status == "abandoned" {
		return fmt.Errorf("ticket is already abandoned")
	}

	// Update ticket
	abandonedAt := time.Now()
	ticket.Status = "abandoned"
	ticket.AbandonedAt = &abandonedAt
	ticket.LastModified = time.Now()

	if ticketReason != "" {
		ticket.Notes = ticketReason
	}

	if dryRun {
		dryRunMsg(fmt.Sprintf("Would mark ticket %s as abandoned", ticketID))
		return nil
	}

	// Save config
	if err := cfgMgr.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	successMsg("Updated configuration")

	// Git commit removed (no longer tracking in git)

	fmt.Println()
	successMsg(fmt.Sprintf("Ticket %s marked as abandoned", ticketID))
	infoMsg("You can archive it with: cctx ticket archive " + ticketID)

	return nil
}

// ticket archive subcommand
var ticketArchiveCmd = &cobra.Command{
	Use:   "archive",
	Short: "Archive completed/abandoned ticket",
	Long: `Archive a completed or abandoned ticket.

Moves ticket to archived directory, removes symlinks from all projects,
and generates documentation for completed tickets.

Specify the ticket using:
  - Flag: cctx -t TICKET-123 ticket archive
  - Env var: export CCTX_TICKET=TICKET-123 && cctx ticket archive`,
	Args: cobra.NoArgs,
	RunE: runTicketArchive,
}

func runTicketArchive(cmd *cobra.Command, args []string) error {
	ticketID := GetTicketIDOrExit()

	// Get data directory
	dataDir := GetDataDirOrExit()

	// Load config
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Find ticket
	ticket := cfg.GetTicket(ticketID, false)
	if ticket == nil {
		return fmt.Errorf("ticket not found: %s", ticketID)
	}

	if ticket.Status == "active" {
		return fmt.Errorf("cannot archive active ticket (mark as completed or abandoned first)")
	}

	fmt.Println()
	infoMsg(fmt.Sprintf("Archiving ticket: %s", ticketID))
	fmt.Println()

	// Confirmation
	if !dryRun {
		if !common.Confirm("Proceed with archive?", false) {
			infoMsg("Operation cancelled")
			return nil
		}
		fmt.Println()
	}

	// Remove symlinks from all linked projects
	for _, lp := range ticket.LinkedProjects {
		project := cfg.GetProject(lp.ContextName)
		if project == nil {
			continue
		}

		symlinkPath := filepath.Join(project.ProjectPath, ticketID+".md")

		if dryRun {
			dryRunMsg(fmt.Sprintf("Would remove symlink from %s", lp.ContextName))
			dryRunMsg(fmt.Sprintf("Would remove from .clauderc"))
		} else {
			if common.FileExists(symlinkPath) {
				if err := common.RemoveSymlink(symlinkPath); err != nil {
					warningMsg(fmt.Sprintf("Failed to remove symlink from %s: %v", lp.ContextName, err))
				} else {
					successMsg(fmt.Sprintf("Removed symlink from %s", lp.ContextName))
				}
			}

			// Update .clauderc
			rcMgr := clauderc.NewManager(project.ProjectPath)
			if err := rcMgr.RemoveFile(ticketID+".md", dryRun); err != nil {
				warningMsg(fmt.Sprintf("Failed to update .clauderc in %s: %v", lp.ContextName, err))
			}
		}
	}

	// Move ticket to archived
	ticketDir := filepath.Join(cfgMgr.GetContextsPath(), "_tickets", ticketID)
	archivedDir := filepath.Join(cfgMgr.GetContextsPath(), "_archived", fmt.Sprintf("%s_%s", time.Now().Format("2006-01-02"), ticketID))

	if dryRun {
		dryRunMsg(fmt.Sprintf("Would move ticket to: %s", archivedDir))
	} else {
		if err := common.EnsureDir(filepath.Dir(archivedDir)); err != nil {
			return fmt.Errorf("failed to create archived directory: %w", err)
		}

		if err := os.Rename(ticketDir, archivedDir); err != nil {
			return fmt.Errorf("failed to move ticket to archived: %w", err)
		}
		successMsg("Moved ticket to archived")
	}

	// Update config: move from active to archived
	ticket.ArchivedPath = fmt.Sprintf("contexts/_archived/%s_%s", time.Now().Format("2006-01-02"), ticketID)

	// Remove from active tickets
	newActive := []config.Ticket{}
	for _, t := range cfg.Tickets.Active {
		if t.TicketID != ticketID {
			newActive = append(newActive, t)
		}
	}
	cfg.Tickets.Active = newActive

	// Add to archived tickets
	cfg.Tickets.Archived = append(cfg.Tickets.Archived, *ticket)

	if !dryRun {
		if err := cfgMgr.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		successMsg("Updated configuration")
	}

	// Git commit removed (no longer tracking in git)

	if !dryRun {
		fmt.Println()
		successMsg(fmt.Sprintf("Successfully archived ticket: %s", ticketID))
		infoMsg(fmt.Sprintf("Location: %s", archivedDir))
	}

	return nil
}

// ticket edit subcommand
var ticketEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit ticket metadata",
	Long: `Edit ticket metadata (title, tags, notes).

Specify the ticket using:
  - Flag: cctx -t TICKET-123 ticket edit --title "New title"
  - Env var: export CCTX_TICKET=TICKET-123 && cctx ticket edit --title "New title"`,
	Args: cobra.NoArgs,
	RunE: runTicketEdit,
}

// ticket delete subcommand
var ticketDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Permanently delete a ticket",
	Long: `Permanently delete a ticket and all associated files.

This removes:
- Ticket from config.json (active or archived)
- Ticket directory and files
- Symlinks from all linked projects
- .clauderc entries in projects

This action cannot be undone.

Specify the ticket using:
  - Flag: cctx -t TICKET-123 ticket delete
  - Env var: export CCTX_TICKET=TICKET-123 && cctx ticket delete`,
	Args: cobra.NoArgs,
	RunE: runTicketDelete,
}

var ticketArchiveAllCmd = &cobra.Command{
	Use:   "archive-all",
	Short: "Archive all active tickets",
	Long: `Archive all active tickets across all projects, or from a specific project.

This command will:
  - Mark all active tickets as completed (if not already)
  - Remove all ticket symlinks from all linked projects
  - Move all tickets to ~/.cctx/contexts/_archived/
  - Update all .clauderc files
  - Update config.json to move tickets from active to archived

This is useful for bulk cleanup or when starting a new sprint/phase.

Specify a project to archive only tickets linked to that project:
  - Flag: cctx -p api-gateway ticket archive-all
  - Env var: export CCTX_PROJECT=api-gateway && cctx ticket archive-all
  - Current directory: cd /path/to/project && cctx ticket archive-all`,
	Args: cobra.NoArgs,
	RunE: runTicketArchiveAll,
}

func runTicketEdit(cmd *cobra.Command, args []string) error {
	ticketID := GetTicketIDOrExit()

	// Check if any flags were provided
	if ticketTitle == "" && ticketTags == "" && ticketNotes == "" {
		return fmt.Errorf("specify at least one field to edit (--title, --tags, or --notes)")
	}

	// Get data directory
	dataDir := GetDataDirOrExit()

	// Load config
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Find ticket
	ticket := cfg.GetTicket(ticketID, true)
	if ticket == nil {
		return fmt.Errorf("ticket not found: %s", ticketID)
	}

	// Update fields
	if ticketTitle != "" {
		ticket.Title = ticketTitle
	}
	if ticketTags != "" {
		ticket.Tags = splitAndTrim(ticketTags, ",")
	}
	if ticketNotes != "" {
		ticket.Notes = ticketNotes
	}
	ticket.LastModified = time.Now()

	if dryRun {
		dryRunMsg(fmt.Sprintf("Would update ticket %s", ticketID))
		return nil
	}

	// Save config
	if err := cfgMgr.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	successMsg("Updated configuration")

	// Git commit removed (no longer tracking in git)

	fmt.Println()
	successMsg(fmt.Sprintf("Successfully updated ticket: %s", ticketID))

	return nil
}

// Helper function to split and trim strings
func splitAndTrim(s string, sep string) []string {
	parts := []string{}
	for _, part := range strings.Split(s, sep) {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

func runTicketDelete(cmd *cobra.Command, args []string) error {
	ticketID := GetTicketIDOrExit()

	// Get data directory
	dataDir := GetDataDirOrExit()

	// Load config
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Find ticket in active or archived
	ticket := cfg.GetTicket(ticketID, true)
	if ticket == nil {
		return fmt.Errorf("ticket not found: %s", ticketID)
	}

	// Determine ticket directory
	var ticketDir string
	if ticket.ArchivedPath != "" {
		ticketDir = filepath.Join(dataDir, ticket.ArchivedPath)
	} else {
		ticketDir = filepath.Join(cfgMgr.GetContextsPath(), "_tickets", ticketID)
	}

	fmt.Println()
	warningMsg(fmt.Sprintf("About to permanently delete ticket: %s", ticketID))
	if ticket.Title != "" {
		infoMsg(fmt.Sprintf("Title: %s", ticket.Title))
	}
	infoMsg(fmt.Sprintf("Status: %s", ticket.Status))
	infoMsg(fmt.Sprintf("Linked projects: %d", len(ticket.LinkedProjects)))
	fmt.Println()
	warningMsg("This will remove:")
	warningMsg("  - Ticket from config.json")
	warningMsg("  - Ticket directory and all files")
	warningMsg("  - Symlinks from all linked projects")
	warningMsg("  - .clauderc entries in projects")
	fmt.Println()
	errorMsg("This action CANNOT be undone!")
	fmt.Println()

	// Confirmation required (unless --force or --dry-run)
	if !ticketForce && !dryRun {
		if !common.Confirm("Are you sure you want to delete this ticket?", false) {
			infoMsg("Operation cancelled")
			return nil
		}
		fmt.Println()
	}

	// Remove symlinks from all linked projects
	for _, lp := range ticket.LinkedProjects {
		project := cfg.GetProject(lp.ContextName)
		if project == nil {
			warningMsg(fmt.Sprintf("Project not found: %s", lp.ContextName))
			continue
		}

		symlinkPath := filepath.Join(project.ProjectPath, ticketID+".md")

		if dryRun {
			dryRunMsg(fmt.Sprintf("Would remove symlink from %s", lp.ContextName))
			dryRunMsg("Would remove from .clauderc")
		} else {
			if common.FileExists(symlinkPath) {
				if err := common.RemoveSymlink(symlinkPath); err != nil {
					warningMsg(fmt.Sprintf("Failed to remove symlink from %s: %v", lp.ContextName, err))
				} else {
					successMsg(fmt.Sprintf("Removed symlink from %s", lp.ContextName))
				}
			}

			// Update .clauderc
			rcMgr := clauderc.NewManager(project.ProjectPath)
			if err := rcMgr.RemoveFile(ticketID+".md", dryRun); err != nil {
				warningMsg(fmt.Sprintf("Failed to update .clauderc in %s: %v", lp.ContextName, err))
			}
		}
	}

	// Remove ticket directory
	if dryRun {
		dryRunMsg(fmt.Sprintf("Would delete directory: %s", ticketDir))
	} else {
		if common.DirExists(ticketDir) {
			if err := os.RemoveAll(ticketDir); err != nil {
				return fmt.Errorf("failed to delete ticket directory: %w", err)
			}
			successMsg("Deleted ticket directory")
		}
	}

	// Remove from config (active or archived)
	found := false
	newActive := []config.Ticket{}
	for _, t := range cfg.Tickets.Active {
		if t.TicketID != ticketID {
			newActive = append(newActive, t)
		} else {
			found = true
		}
	}
	cfg.Tickets.Active = newActive

	if !found {
		newArchived := []config.Ticket{}
		for _, t := range cfg.Tickets.Archived {
			if t.TicketID != ticketID {
				newArchived = append(newArchived, t)
			}
		}
		cfg.Tickets.Archived = newArchived
	}

	if !dryRun {
		if err := cfgMgr.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		successMsg("Updated configuration")
	}

	// Git commit removed (no longer tracking in git)

	if !dryRun {
		fmt.Println()
		successMsg(fmt.Sprintf("Successfully deleted ticket: %s", ticketID))
	}

	return nil
}

func runTicketArchiveAll(cmd *cobra.Command, args []string) error {
	// Get data directory
	dataDir := GetDataDirOrExit()

	// Load config
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if we should filter by project
	var projectFilter string
	if projectFlag != "" || os.Getenv("CCTX_PROJECT") != "" {
		projectFilter, err = GetProjectContext(dataDir)
		if err != nil {
			return err
		}

		// Verify project exists
		if cfg.GetProject(projectFilter) == nil {
			return fmt.Errorf("project not found: %s", projectFilter)
		}
	}

	// Filter tickets by project if specified
	ticketsToArchive := []config.Ticket{}
	if projectFilter != "" {
		// Only archive tickets linked to this project
		for _, ticket := range cfg.Tickets.Active {
			for _, lp := range ticket.LinkedProjects {
				if lp.ContextName == projectFilter {
					ticketsToArchive = append(ticketsToArchive, ticket)
					break
				}
			}
		}
	} else {
		// Archive all active tickets
		ticketsToArchive = cfg.Tickets.Active
	}

	// Check if there are any tickets to archive
	if len(ticketsToArchive) == 0 {
		if projectFilter != "" {
			infoMsg(fmt.Sprintf("No active tickets to archive for project: %s", projectFilter))
		} else {
			infoMsg("No active tickets to archive")
		}
		return nil
	}

	// Show what will be archived
	fmt.Println()
	if projectFilter != "" {
		infoMsg(fmt.Sprintf("Project: %s", projectFilter))
		infoMsg(fmt.Sprintf("Found %d active ticket(s) to archive:", len(ticketsToArchive)))
	} else {
		infoMsg(fmt.Sprintf("Found %d active ticket(s) to archive:", len(ticketsToArchive)))
	}
	fmt.Println()
	for _, ticket := range ticketsToArchive {
		fmt.Printf("  • %s", ticket.TicketID)
		if ticket.Title != "" {
			fmt.Printf(" - %s", ticket.Title)
		}
		fmt.Printf(" (%d project(s))\n", len(ticket.LinkedProjects))
	}
	fmt.Println()
	infoMsg("This will:")
	infoMsg("  - Remove all ticket symlinks from all projects")
	infoMsg("  - Update all .clauderc files")
	infoMsg("  - Move tickets to ~/.cctx/contexts/_archived/")
	fmt.Println()

	// Confirmation required (unless --force or --dry-run)
	if !ticketForce && !dryRun {
		confirmMsg := "Archive all active tickets?"
		if projectFilter != "" {
			confirmMsg = fmt.Sprintf("Archive all active tickets for project '%s'?", projectFilter)
		}
		if !common.Confirm(confirmMsg, false) {
			infoMsg("Operation cancelled")
			return nil
		}
		fmt.Println()
	}

	archivedCount := 0
	for _, ticket := range ticketsToArchive {
		ticketID := ticket.TicketID

		if verbose {
			infoMsg(fmt.Sprintf("Archiving ticket: %s", ticketID))
		}

		// Remove symlinks from all linked projects
		for _, lp := range ticket.LinkedProjects {
			project := cfg.GetProject(lp.ContextName)
			if project == nil {
				if verbose {
					warningMsg(fmt.Sprintf("Project not found: %s", lp.ContextName))
				}
				continue
			}

			symlinkPath := filepath.Join(project.ProjectPath, ticketID+".md")

			if dryRun {
				dryRunMsg(fmt.Sprintf("Would remove symlink from %s: %s", lp.ContextName, ticketID))
			} else {
				if common.FileExists(symlinkPath) {
					if err := common.RemoveSymlink(symlinkPath); err != nil {
						warningMsg(fmt.Sprintf("Failed to remove symlink from %s: %v", lp.ContextName, err))
					} else if verbose {
						successMsg(fmt.Sprintf("Removed %s symlink from %s", ticketID, lp.ContextName))
					}
				}

				// Update .clauderc
				rcMgr := clauderc.NewManager(project.ProjectPath)
				if err := rcMgr.RemoveFile(ticketID+".md", dryRun); err != nil {
					warningMsg(fmt.Sprintf("Failed to update .clauderc in %s: %v", lp.ContextName, err))
				}
			}
		}

		// Move ticket directory to archived
		ticketDir := filepath.Join(cfgMgr.GetContextsPath(), "_tickets", ticketID)
		archivedDir := filepath.Join(cfgMgr.GetContextsPath(), "_archived",
			fmt.Sprintf("%s_%s", time.Now().Format("2006-01-02"), ticketID))

		if dryRun {
			dryRunMsg(fmt.Sprintf("Would move %s to archived", ticketID))
		} else {
			if common.DirExists(ticketDir) {
				// Ensure archived directory exists
				if err := common.EnsureDir(filepath.Join(cfgMgr.GetContextsPath(), "_archived")); err != nil {
					return fmt.Errorf("failed to create archived directory: %w", err)
				}

				if err := os.Rename(ticketDir, archivedDir); err != nil {
					warningMsg(fmt.Sprintf("Failed to move %s to archived: %v", ticketID, err))
					continue
				}
			}
		}

		archivedCount++
		if !verbose {
			successMsg(fmt.Sprintf("Archived: %s", ticketID))
		}
	}

	if !dryRun {
		// Create a map of archived ticket IDs for quick lookup
		archivedTicketIDs := make(map[string]bool)
		for _, ticket := range ticketsToArchive {
			archivedTicketIDs[ticket.TicketID] = true
		}

		// Move archived tickets from active to archived in config
		newActive := []config.Ticket{}
		for i := range cfg.Tickets.Active {
			ticket := &cfg.Tickets.Active[i]
			if archivedTicketIDs[ticket.TicketID] {
				// This ticket should be archived
				if ticket.Status != "completed" && ticket.Status != "abandoned" {
					ticket.Status = "completed"
				}
				ticket.ArchivedPath = fmt.Sprintf("contexts/_archived/%s_%s",
					time.Now().Format("2006-01-02"), ticket.TicketID)
				cfg.Tickets.Archived = append(cfg.Tickets.Archived, *ticket)
			} else {
				// Keep this ticket active
				newActive = append(newActive, *ticket)
			}
		}
		cfg.Tickets.Active = newActive

		if err := cfgMgr.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
	}

	// Git commit removed (no longer tracking in git)

	if !dryRun {
		fmt.Println()
		successMsg(fmt.Sprintf("Successfully archived %d ticket(s)", archivedCount))
	}

	return nil
}

// getDefaultSessionsContent returns the default content for SESSIONS.md
func getDefaultSessionsContent() string {
	return `# Development Sessions

This file tracks interactions with Claude Code during ticket development.

---

## Session Template (copy for new sessions)

` + "```markdown" + `
## Session YYYY-MM-DD: [Brief Title]

### Summary
[One sentence summary]

### What Was Done
-

### Key Decisions
- **Decision**:
  - **Rationale**:

### Issues Resolved
-

### Files Changed
-

### Commands Used
` + "```bash" + `

` + "```" + `

### Notes for Next Session
- [ ]
` + "```" + `

---

## Usage Instructions

1. Copy the template above for each new session
2. Fill in the date and title
3. Document what was accomplished
4. Note key decisions and rationale
5. List issues that were resolved
6. Track files that were changed
7. Include important commands used
8. Add any notes for the next session

---
`
}
