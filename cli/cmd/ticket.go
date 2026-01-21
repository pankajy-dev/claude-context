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
	Use:   "create <ticket-id>",
	Short: "Create a new ticket workspace",
	Long: `Create a new ticket workspace for tracking work across multiple projects.

Automatically creates a symlink in the current directory and updates .clauderc
to include the ticket file.`,
	Args: cobra.ExactArgs(1),
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
}

func runTicketCreate(cmd *cobra.Command, args []string) error {
	ticketID := args[0]

	// Get data directory
	dataDir := GetDataDirOrExit()

	// Load config
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if ticket already exists
	if cfg.GetTicket(ticketID, true) != nil {
		return fmt.Errorf("ticket already exists: %s", ticketID)
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

## Summary

Brief description here.

---

## Source & Requirements

**Jira/Issue Link:**

**Description:**

**Acceptance Criteria:**
- [ ]
- [ ]
- [ ]

**Priority:** High/Medium/Low

**Tags:**

**Services/Repos:**
-

---

## Research & Context

**Background:**

**Investigation Notes:**

**Related Work:**
-

---

## Discussions

### YYYY-MM-DD - [Person/Team]

**Topic:**

**Decisions:**
-

**Actions:**
- [ ]

---

## Technical Approach

**Solution:**

**Files to Change:**
-

**Considerations:**
-

---

## Implementation

- [ ]
- [ ]
- [ ] Run tests
- [ ] Update docs

---

## Testing

**Test Cases:**
- [ ]
- [ ]

**Rollback:**

---

## Work Log

### YYYY-MM-DD

**Done:**
-

**Blockers:**
-

**Next:**
-

---

## References

-

`, ticketID)
		if err := os.WriteFile(ticketFile, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to create ticket file: %w", err)
		}
		successMsg("Created ticket.md")
	}

	// Parse tags
	var tags []string
	if ticketTags != "" {
		tags = splitAndTrim(ticketTags, ",")
	}

	// Create ticket metadata
	ticket := config.Ticket{
		TicketID:       ticketID,
		Title:          ticketTitle,
		Status:         "active",
		CreatedAt:      time.Now(),
		LastModified:   time.Now(),
		LinkedProjects: []config.LinkedProject{},
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

	if dryRun {
		dryRunMsg(fmt.Sprintf("Would create symlink: %s", symlinkPath))
		dryRunMsg(fmt.Sprintf("Would add '%s' to .clauderc", symlinkName))
	} else {
		if common.FileExists(symlinkPath) {
			warningMsg(fmt.Sprintf("File already exists: %s", symlinkPath))
		} else {
			if err := common.CreateSymlink(ticketFile, symlinkPath); err != nil {
				return fmt.Errorf("failed to create symlink: %w", err)
			}
			successMsg(fmt.Sprintf("Created symlink: %s", symlinkName))

			// Update .clauderc
			rcMgr := clauderc.NewManager(currentDir)
			if err := rcMgr.AddFile(symlinkName, dryRun); err != nil {
				warningMsg(fmt.Sprintf("Failed to update .clauderc: %v", err))
			} else {
				successMsg("Updated .clauderc")
			}
		}
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
		fmt.Println()
		infoMsg("Next steps:")
		infoMsg(fmt.Sprintf("  1. Edit ticket context: vim %s", symlinkName))
		infoMsg(fmt.Sprintf("  2. (Optional) Link to other projects: cctx ticket link %s <project>", ticketID))
	}

	return nil
}

// ticket link subcommand
var ticketLinkCmd = &cobra.Command{
	Use:   "link <ticket-id> <context-name> [<context-name>...]",
	Short: "Link ticket to one or more projects",
	Long: `Link a ticket to one or more projects.

Creates symlinks in project directories and updates .clauderc automatically.`,
	Args: cobra.MinimumNArgs(2),
	RunE: runTicketLink,
}

func runTicketLink(cmd *cobra.Command, args []string) error {
	ticketID := args[0]
	projectNames := args[1:]

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

	// Git commit
	if successCount > 0 {
		commitMsg := fmt.Sprintf("Link ticket %s to projects", ticketID)
		if err := common.GitCommit(dataDir, commitMsg, dryRun); err != nil {
			warningMsg(fmt.Sprintf("Failed to commit to git: %v", err))
		} else if !dryRun {
			successMsg("Committed changes to git")
		}
	}

	if !dryRun {
		fmt.Println()
		successMsg(fmt.Sprintf("Linked to %d project(s)", successCount))
	}

	return nil
}

// ticket unlink subcommand
var ticketUnlinkCmd = &cobra.Command{
	Use:   "unlink <ticket-id> [<context-name>...]",
	Short: "Unlink ticket from project(s)",
	Long: `Unlink a ticket from one or more projects.

Removes symlinks and updates .clauderc automatically.
Use --all to unlink from all projects.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runTicketUnlink,
}

func runTicketUnlink(cmd *cobra.Command, args []string) error {
	ticketID := args[0]
	projectNames := []string{}
	if !ticketAll {
		if len(args) < 2 {
			return fmt.Errorf("specify project names or use --all")
		}
		projectNames = args[1:]
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
	ticket := cfg.GetTicket(ticketID, false)
	if ticket == nil {
		return fmt.Errorf("ticket not found: %s", ticketID)
	}

	// If --all, get all linked projects
	if ticketAll {
		for _, lp := range ticket.LinkedProjects {
			projectNames = append(projectNames, lp.ContextName)
		}
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

	// Git commit
	if successCount > 0 {
		commitMsg := fmt.Sprintf("Unlink ticket %s from projects", ticketID)
		if err := common.GitCommit(dataDir, commitMsg, dryRun); err != nil {
			warningMsg(fmt.Sprintf("Failed to commit to git: %v", err))
		} else if !dryRun {
			successMsg("Committed changes to git")
		}
	}

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

	if len(tickets) == 0 {
		infoMsg(fmt.Sprintf("No tickets found (status: %s)", ticketStatus))
		return nil
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
	Use:   "show <ticket-id>",
	Short: "Show detailed ticket information",
	Long:  `Show detailed information about a specific ticket.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runTicketShow,
}

func runTicketShow(cmd *cobra.Command, args []string) error {
	ticketID := args[0]

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
	Use:   "complete <ticket-id>",
	Short: "Mark ticket as completed",
	Long:  `Mark a ticket as completed with optional commit hashes and PR numbers.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runTicketComplete,
}

func runTicketComplete(cmd *cobra.Command, args []string) error {
	ticketID := args[0]

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

	// Parse commits
	if ticketCommits != "" {
		commits := []string{}
		for _, c := range splitAndTrim(ticketCommits, ",") {
			commits = append(commits, c)
		}
		ticket.Commits = commits
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
		return nil
	}

	// Save config
	if err := cfgMgr.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	successMsg("Updated configuration")

	// Git commit
	commitMsg := fmt.Sprintf("Complete ticket: %s", ticketID)
	if err := common.GitCommit(dataDir, commitMsg, dryRun); err != nil {
		warningMsg(fmt.Sprintf("Failed to commit to git: %v", err))
	} else {
		successMsg("Committed changes to git")
	}

	fmt.Println()
	successMsg(fmt.Sprintf("Ticket %s marked as completed", ticketID))
	infoMsg("Next step: archive the ticket with: cctx ticket archive " + ticketID)

	return nil
}

// ticket abandon subcommand
var ticketAbandonCmd = &cobra.Command{
	Use:   "abandon <ticket-id>",
	Short: "Mark ticket as abandoned",
	Long:  `Mark a ticket as abandoned with optional reason.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runTicketAbandon,
}

func runTicketAbandon(cmd *cobra.Command, args []string) error {
	ticketID := args[0]

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

	// Git commit
	commitMsg := fmt.Sprintf("Abandon ticket: %s", ticketID)
	if err := common.GitCommit(dataDir, commitMsg, dryRun); err != nil {
		warningMsg(fmt.Sprintf("Failed to commit to git: %v", err))
	} else {
		successMsg("Committed changes to git")
	}

	fmt.Println()
	successMsg(fmt.Sprintf("Ticket %s marked as abandoned", ticketID))
	infoMsg("You can archive it with: cctx ticket archive " + ticketID)

	return nil
}

// ticket archive subcommand
var ticketArchiveCmd = &cobra.Command{
	Use:   "archive <ticket-id>",
	Short: "Archive completed/abandoned ticket",
	Long: `Archive a completed or abandoned ticket.

Moves ticket to archived directory, removes symlinks from all projects,
and generates documentation for completed tickets.`,
	Args: cobra.ExactArgs(1),
	RunE: runTicketArchive,
}

func runTicketArchive(cmd *cobra.Command, args []string) error {
	ticketID := args[0]

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
	Use:   "edit <ticket-id>",
	Short: "Edit ticket metadata",
	Long:  `Edit ticket metadata (title, tags, notes).`,
	Args:  cobra.ExactArgs(1),
	RunE:  runTicketEdit,
}

// ticket delete subcommand
var ticketDeleteCmd = &cobra.Command{
	Use:   "delete <ticket-id>",
	Short: "Permanently delete a ticket",
	Long: `Permanently delete a ticket and all associated files.

This removes:
- Ticket from config.json (active or archived)
- Ticket directory and files
- Symlinks from all linked projects
- .clauderc entries in projects

This action cannot be undone.`,
	Args: cobra.ExactArgs(1),
	RunE: runTicketDelete,
}

func runTicketEdit(cmd *cobra.Command, args []string) error {
	ticketID := args[0]

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

	// Git commit
	commitMsg := fmt.Sprintf("Edit ticket: %s", ticketID)
	if err := common.GitCommit(dataDir, commitMsg, dryRun); err != nil {
		warningMsg(fmt.Sprintf("Failed to commit to git: %v", err))
	} else {
		successMsg("Committed changes to git")
	}

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
	ticketID := args[0]

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
