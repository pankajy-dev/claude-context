package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/pankaj/claude-context/internal/common"
	"github.com/pankaj/claude-context/internal/config"
	"github.com/spf13/cobra"
)

var ticketCmd = &cobra.Command{
	Use:   "ticket",
	Short: "Manage ticket workspaces",
	Long:  `Create and manage ticket workspaces for tracking work across multiple projects.`,
	// Runnable with NoArgs so a removed verb like "ticket archive" is reported
	// as an unknown command instead of silently printing help.
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error { return cmd.Help() },
}

// ticket create subcommand
var ticketCreateCmd = &cobra.Command{
	Use:   "create [<ticket-id>]",
	Short: "Create a new ticket workspace",
	Long: `Create a new ticket workspace for tracking work across multiple projects.

Creates the ticket file in the current directory and a symlink in the data
directory.

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
	ticketCreateCmd.InheritedFlags().MarkHidden("ticket")
	ticketCreateCmd.InheritedFlags().MarkHidden("project")

	// ticket link
	ticketCmd.AddCommand(ticketLinkCmd)
	ticketLinkCmd.InheritedFlags().MarkHidden("ticket")

	// ticket unlink
	ticketCmd.AddCommand(ticketUnlinkCmd)
	ticketUnlinkCmd.Flags().BoolVar(&ticketAll, "all", false, "Unlink from all projects")
	ticketUnlinkCmd.InheritedFlags().MarkHidden("ticket")

	// ticket list
	ticketCmd.AddCommand(ticketListCmd)
	ticketListCmd.Flags().StringVar(&ticketStatus, "status", "active", "Filter by status (active|completed|abandoned|archived|all)")
	ticketListCmd.InheritedFlags().MarkHidden("ticket")

	// ticket show
	ticketCmd.AddCommand(ticketShowCmd)
	ticketShowCmd.InheritedFlags().MarkHidden("ticket")

	// ticket complete
	ticketCmd.AddCommand(ticketCompleteCmd)
	ticketCompleteCmd.Flags().StringVar(&ticketCommits, "commits", "", "Comma-separated commit hashes")
	ticketCompleteCmd.Flags().StringVar(&ticketPRs, "prs", "", "Comma-separated PR numbers")
	ticketCompleteCmd.InheritedFlags().MarkHidden("ticket")

	// ticket abandon
	ticketCmd.AddCommand(ticketAbandonCmd)
	ticketAbandonCmd.Flags().StringVar(&ticketReason, "reason", "", "Reason for abandoning")
	ticketAbandonCmd.InheritedFlags().MarkHidden("ticket")

	// ticket restore
	ticketCmd.AddCommand(ticketRestoreCmd)
	ticketRestoreCmd.Flags().BoolVarP(&ticketForce, "force", "f", false, "Skip confirmation prompt")
	ticketRestoreCmd.InheritedFlags().MarkHidden("ticket")

	// ticket edit
	ticketCmd.AddCommand(ticketEditCmd)
	ticketEditCmd.Flags().StringVar(&ticketTitle, "title", "", "Update title")
	ticketEditCmd.Flags().StringVar(&ticketTags, "tags", "", "Update tags")
	ticketEditCmd.Flags().StringVar(&ticketNotes, "notes", "", "Update notes")
	ticketEditCmd.InheritedFlags().MarkHidden("ticket")

	// ticket delete
	ticketCmd.AddCommand(ticketDeleteCmd)
	ticketDeleteCmd.Flags().BoolVarP(&ticketForce, "force", "f", false, "Skip confirmation prompt")
	ticketDeleteCmd.InheritedFlags().MarkHidden("ticket")

	// ticket working
	ticketCmd.AddCommand(ticketWorkingCmd)
	ticketWorkingCmd.InheritedFlags().MarkHidden("ticket")
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

	// Try to detect current project for context
	currentProject, _ := GetProjectContext(dataDir)

	// Working entries for archived/finished tickets can never be completed again,
	// so drop them before prompting instead of asking about them every time.
	if !dryRun {
		if pruned := pruneStaleWorkingTickets(cfg); len(pruned) > 0 {
			if err := cfgMgr.Save(cfg); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}
			infoMsg(fmt.Sprintf("Removed stale working ticket(s): %s", strings.Join(pruned, ", ")))
		}
	}

	// Check for existing working tickets (cross-repo support)
	if len(cfg.CurrentWorkingTickets) > 0 {
		// Determine what ticket we're about to create/work on
		var tentativeTicketID string
		if len(args) > 0 {
			tentativeTicketID = args[0]
		} else {
			// Try to detect from branch
			branch := common.GetGitBranch()
			if branch != "" && branch != "main" && branch != "master" {
				tentativeTicketID = branch
			}
		}

		// Check if this ticket is already in the working list
		alreadyWorking := cfg.IsCurrentlyWorking(tentativeTicketID)

		// If starting a NEW ticket, show existing working tickets
		if tentativeTicketID != "" && !alreadyWorking {
			if !dryRun {
				fmt.Println()
				warningMsg(fmt.Sprintf("You have %d active ticket(s):", len(cfg.CurrentWorkingTickets)))
				fmt.Println()
				for _, cwt := range cfg.CurrentWorkingTickets {
					ticket := cfg.GetTicket(cwt.TicketID, false)
					status := "unknown"
					if ticket != nil {
						status = ticket.Status
					}
					fmt.Printf("  - %s", cwt.TicketID)
					if cwt.ProjectName != "" {
						fmt.Printf(" (%s)", cwt.ProjectName)
					}
					fmt.Printf(" [%s]", status)
					fmt.Println()
				}
				fmt.Println()
				fmt.Println("Do you want to complete any tickets before starting " + tentativeTicketID + "?")
				fmt.Println()

				// Build numbered list: one entry per active ticket + "all" + continue + cancel
				ticketChoices := cfg.CurrentWorkingTickets
				for i, cwt := range ticketChoices {
					ticket := cfg.GetTicket(cwt.TicketID, false)
					status := "unknown"
					if ticket != nil {
						status = ticket.Status
					}
					label := cwt.TicketID
					if cwt.ProjectName != "" {
						label += fmt.Sprintf(" (%s)", cwt.ProjectName)
					}
					fmt.Printf("  %d. Mark %s as completed [%s]\n", i+1, label, status)
				}
				allChoice := len(ticketChoices) + 1
				continueChoice := len(ticketChoices) + 2
				cancelChoice := len(ticketChoices) + 3
				fmt.Printf("  %d. Mark all as completed\n", allChoice)
				fmt.Printf("  %d. No, continue with multiple active tickets\n", continueChoice)
				fmt.Printf("  %d. Cancel\n", cancelChoice)
				fmt.Printf("\nEnter choice [1-%d]: ", cancelChoice)

				var choice string
				fmt.Scanln(&choice)
				fmt.Println()

				choiceNum := 0
				fmt.Sscanf(choice, "%d", &choiceNum)

				markComplete := func(ticketToComplete string) {
					if workingTicketIsStale(cfg, ticketToComplete) {
						if err := dropStaleWorkingEntry(cfgMgr, cfg, ticketToComplete); err != nil {
							warningMsg(err.Error())
						}
						return
					}
					if err := runTicketComplete(ticketCompleteCmd, []string{ticketToComplete}); err != nil {
						warningMsg(fmt.Sprintf("Failed to complete ticket %s: %v", ticketToComplete, err))
					}
					// Reload config since runTicketComplete modifies it
					cfg, _ = cfgMgr.Load()
				}

				switch {
				case choiceNum >= 1 && choiceNum <= len(ticketChoices):
					markComplete(ticketChoices[choiceNum-1].TicketID)
					fmt.Println()
				case choiceNum == allChoice:
					for _, cwt := range ticketChoices {
						markComplete(cwt.TicketID)
					}
					fmt.Println()
				case choiceNum == continueChoice:
					infoMsg("Continuing with multiple active tickets")
					fmt.Println()
				case choiceNum == cancelChoice || choice == "":
					infoMsg("Operation cancelled")
					return nil
				default:
					return fmt.Errorf("invalid choice: %s", choice)
				}
			} else {
				warningMsg(fmt.Sprintf("Would prompt about %d working ticket(s)", len(cfg.CurrentWorkingTickets)))
			}
		}
	}

	// Determine ticket ID: from args or auto-detect
	if len(args) > 0 {
		ticketID = args[0]
		if existingTicket := cfg.GetTicket(ticketID, true); existingTicket != nil {
			fmt.Println()
			warningMsg(fmt.Sprintf("Ticket '%s' already exists", ticketID))
			if existingTicket.Title != "" {
				infoMsg(fmt.Sprintf("Title: %s", existingTicket.Title))
			}
			if existingTicket.ArchivedPath != "" {
				infoMsg(fmt.Sprintf("It is archived; to continue it run: cctx ticket restore %s", ticketID))
			}
			fmt.Println()

			if dryRun {
				warningMsg("In dry-run mode, would prompt user for action")
			} else {
				projectScopedID := ""
				if currentProject != "" && cfg.GetTicket(currentProject+"-"+ticketID, true) == nil {
					projectScopedID = fmt.Sprintf("%s-%s", currentProject, ticketID)
				}

				fmt.Println("Choose an option:")
				if projectScopedID != "" {
					fmt.Printf("  1. Create project-scoped ticket: %s\n", projectScopedID)
				}
				fmt.Printf("  2. Create with auto-incremented suffix (e.g., %s-1)\n", ticketID)
				fmt.Println("  3. Cancel")
				fmt.Print("\nEnter choice [1-3]: ")

				var choice string
				fmt.Scanln(&choice)
				fmt.Println()

				switch choice {
				case "1":
					if projectScopedID == "" {
						return fmt.Errorf("invalid choice: %s", choice)
					}
					ticketID = projectScopedID
					infoMsg(fmt.Sprintf("Creating project-scoped ticket: %s", ticketID))
				case "2":
					originalID := ticketID
					for suffix := 1; cfg.GetTicket(ticketID, true) != nil; suffix++ {
						ticketID = fmt.Sprintf("%s-%d", originalID, suffix)
					}
					infoMsg(fmt.Sprintf("Creating ticket with suffix: %s", ticketID))
				case "3", "":
					infoMsg("Operation cancelled")
					return nil
				default:
					return fmt.Errorf("invalid choice: %s", choice)
				}
			}
		}
	} else {
		// Auto-detect from branch; feature/CBP-1 becomes CBP-1
		branch := common.GetGitBranch()
		if branch == "" || branch == "main" || branch == "master" {
			return fmt.Errorf("cannot auto-detect ticket ID: not on a feature branch (current: %s)", branch)
		}
		ticketID = branch[strings.LastIndex(branch, "/")+1:]
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

	if !validTicketID(ticketID) {
		return fmt.Errorf("invalid ticket ID %q: use letters, digits, '.', '_' and '-' only", ticketID)
	}

	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	workspaceRoot, err := ensureWorkspaceDir(currentDir, dataDir)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(filepath.Join(workspaceRoot, ticketID)); err == nil {
		return fmt.Errorf("workspace already exists: %s", filepath.Join(workspaceRoot, ticketID))
	}

	var tags []string
	if ticketTags != "" {
		tags = splitAndTrim(ticketTags, ",")
	}

	if dryRun {
		dryRunMsg(fmt.Sprintf("Would create ticket workspace: %s", filepath.Join(workspaceRoot, ticketID)))
		return nil
	}

	workspace, err := scaffoldTicketWorkspace(workspaceRoot, ticketID, dataDir)
	if err != nil {
		return err
	}

	projectName := currentProject
	if projectName == "" {
		projectName = filepath.Base(currentDir)
	}
	cfg.Tickets.Active = append(cfg.Tickets.Active, config.Ticket{
		TicketID:      ticketID,
		Title:         ticketTitle,
		Status:        "active",
		CreatedAt:     time.Now(),
		LastModified:  time.Now(),
		Tags:          tags,
		Notes:         ticketNotes,
		WorkspacePath: canon(workspace),
	})
	cfg.AddCurrentWorkingTicket(ticketID, projectName)
	if err := cfgMgr.Save(cfg); err != nil {
		os.RemoveAll(workspace)
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println()
	successMsg(fmt.Sprintf("Successfully created ticket: %s", ticketID))
	if ticketTitle != "" {
		infoMsg(fmt.Sprintf("Title: %s", ticketTitle))
	}
	infoMsg(fmt.Sprintf("Workspace: %s", workspace))
	for _, f := range ticketTemplateFiles {
		fmt.Printf("  - %s\n", f.file)
	}
	fmt.Println()
	infoMsg("Next step: ask Claude to fill in requirements.md (or use your own SDD skill in this directory)")

	return nil
}

// ticket link subcommand
var ticketLinkCmd = &cobra.Command{
	Use:   "link <ticket-id> [project...]",
	Short: "Link ticket to one or more projects",
	Long: `Link a ticket to one or more projects.

Creates symlinks in project directories.

With no arguments the ticket ID comes from --ticket/-t, CCTX_TICKET or the
current git branch, and the project from --project/-p, CCTX_PROJECT or the
current directory.

Examples:
  cctx ticket link TICKET-123 api-gateway frontend
  cctx ticket link TICKET-123 --project api-gateway
  cctx ticket link TICKET-123          # Auto-detects current project
  cctx ticket link                     # Auto-detects both
  cctx -p frontend ticket link         # Auto-detects the ticket`,
	Args: cobra.ArbitraryArgs,
	RunE: runTicketLink,
}

func runTicketLink(cmd *cobra.Command, args []string) error {
	// Ticket ID comes from the first argument, or the usual flag/env/branch chain.
	// Only an empty arg list falls back, so a lone argument is never mistaken
	// for a project name.
	var ticketID string
	if len(args) > 0 {
		ticketID = args[0]
		args = args[1:]
	} else {
		ticketID = GetTicketID()
		if ticketID == "" {
			return fmt.Errorf("no ticket specified. Pass a ticket ID, use --ticket/-t, set CCTX_TICKET, or checkout a feature branch")
		}
		infoMsg(fmt.Sprintf("Using ticket: %s", ticketID))
	}

	// Get data directory
	dataDir := GetDataDirOrExit()

	// Determine project names from remaining args
	projectNames := []string{}

	if len(args) > 0 {
		// Projects specified as additional arguments
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

	if ticket.WorkspacePath != "" {
		return fmt.Errorf("directory tickets live in %s and cannot be linked to other projects", ticket.WorkspacePath)
	}

	if ticket.Status != "active" {
		return fmt.Errorf("ticket is not active (status: %s)", ticket.Status)
	}

	ticketFile := filepath.Join(cfgMgr.GetContextsPath(), "_tickets", ticketID, ticketID+".md")

	// Set primary context if not set
	if ticket.PrimaryContextName == "" && len(projectNames) > 0 {
		ticket.PrimaryContextName = projectNames[0]
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
		sessionsSymlinkPath := filepath.Join(project.ProjectPath, "SESSIONS.md")

		// Determine target based on primary/secondary
		var ticketTarget, sessionsTarget string
		if ticket.PrimaryContextName == projectName {
			// Primary project - move concrete files here
			ticketTarget = symlinkPath // Will be concrete file, not symlink
			sessionsTarget = sessionsSymlinkPath
		} else {
			// Secondary project - symlink to primary
			primaryProject := cfg.GetProject(ticket.PrimaryContextName)
			if primaryProject != nil && common.FileExists(filepath.Join(primaryProject.ProjectPath, ticketID+".md")) {
				ticketTarget = filepath.Join(primaryProject.ProjectPath, ticketID+".md")
				sessionsTarget = filepath.Join(primaryProject.ProjectPath, "SESSIONS.md")
			} else {
				// Fallback to data dir if primary not accessible
				ticketTarget = ticketFile
				sessionsFile := filepath.Join(cfgMgr.GetContextsPath(), "_tickets", ticketID, "SESSIONS.md")
				sessionsTarget = sessionsFile
			}
		}

		if dryRun {
			dryRunMsg(fmt.Sprintf("Would create symlink: %s", symlinkPath))
		} else {
			// For primary project, ensure concrete files are in project
			if ticket.PrimaryContextName == projectName {
				switch {
				case common.FileExists(symlinkPath) && !common.IsSymlink(symlinkPath):
					// Files already exist in primary project (new v2 tickets)
					successMsg("Concrete files already in primary project")

				case common.FileExists(symlinkPath):
					// Symlink that still resolves (project used to be secondary)
					infoMsg("Ticket file already reachable via symlink")

				case common.FileExists(ticketFile) && !common.IsSymlink(ticketFile):
					// Migrate from old location (data dir)
					if err := common.CopyFile(ticketFile, symlinkPath); err != nil {
						errorMsg(fmt.Sprintf("Failed to copy ticket file: %v", err))
						continue
					}
					os.Remove(ticketFile)
					// Create symlink in data dir pointing to project
					if err := common.CreateSymlink(symlinkPath, ticketFile); err != nil {
						warningMsg(fmt.Sprintf("Failed to create data dir symlink: %v", err))
					}
					successMsg("Migrated ticket file to primary project")

					sessionsFile := filepath.Join(cfgMgr.GetContextsPath(), "_tickets", ticketID, "SESSIONS.md")
					if common.FileExists(sessionsFile) && !common.IsSymlink(sessionsFile) {
						if err := common.CopyFile(sessionsFile, sessionsSymlinkPath); err != nil {
							warningMsg(fmt.Sprintf("Failed to copy sessions file: %v", err))
						} else {
							os.Remove(sessionsFile)
							if err := common.CreateSymlink(sessionsSymlinkPath, sessionsFile); err != nil {
								warningMsg(fmt.Sprintf("Failed to create data dir sessions symlink: %v", err))
							}
							successMsg("Migrated sessions file to primary project")
						}
					}

				default:
					// No file to link — don't record a link
					errorMsg(fmt.Sprintf("No ticket file found for %s (looked in %s and %s)", ticketID, symlinkPath, ticketFile))
					continue
				}
			} else {
				// Secondary project - create symlink to target
				if err := common.CreateSymlink(ticketTarget, symlinkPath); err != nil {
					errorMsg(fmt.Sprintf("Failed to create symlink: %v", err))
					continue
				}
				successMsg("Created symlink")

				// Create sessions symlink if target exists
				if common.FileExists(sessionsTarget) {
					if err := common.CreateSymlink(sessionsTarget, sessionsSymlinkPath); err != nil {
						warningMsg(fmt.Sprintf("Failed to create sessions symlink: %v", err))
					} else {
						successMsg("Created sessions symlink")
					}
				}
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
	Use:   "unlink <ticket-id> [project...]",
	Short: "Unlink ticket from project(s)",
	Long: `Unlink a ticket from one or more projects.

Removes symlinks automatically.

With no arguments the ticket ID comes from --ticket/-t, CCTX_TICKET or the
current git branch.

Examples:
  cctx ticket unlink TICKET-123 api-gateway frontend
  cctx ticket unlink TICKET-123 --project api-gateway
  cctx ticket unlink TICKET-123  # Auto-detects current project
  cctx ticket unlink              # Auto-detects both
  cctx ticket unlink TICKET-123 --all  # Unlink from all projects`,
	Args: cobra.ArbitraryArgs,
	RunE: runTicketUnlink,
}

func runTicketUnlink(cmd *cobra.Command, args []string) error {
	// Ticket ID from the first argument, or the flag/env/branch chain
	var ticketID string
	if len(args) > 0 {
		ticketID = args[0]
		args = args[1:]
	} else {
		ticketID = GetTicketID()
		if ticketID == "" {
			return fmt.Errorf("no ticket specified. Pass a ticket ID, use --ticket/-t, set CCTX_TICKET, or checkout a feature branch")
		}
		infoMsg(fmt.Sprintf("Using ticket: %s", ticketID))
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
	if ticket.WorkspacePath != "" {
		return fmt.Errorf("directory tickets live in %s and cannot be linked to other projects", ticket.WorkspacePath)
	}

	// Determine project names from remaining args
	projectNames := []string{}

	if ticketAll {
		// --all flag: unlink from all linked projects
		for _, lp := range ticket.LinkedProjects {
			projectNames = append(projectNames, lp.ContextName)
		}
	} else if len(args) > 0 {
		// Projects specified as additional arguments
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

		ticketFilePath := filepath.Join(project.ProjectPath, ticketID+".md")
		sessionsFilePath := filepath.Join(project.ProjectPath, "SESSIONS.md")

		if dryRun {
			dryRunMsg(fmt.Sprintf("Would remove: %s", ticketFilePath))
		} else {
			// Handle primary vs secondary project
			isPrimary := ticket.PrimaryContextName == projectName

			if isPrimary {
				// Primary project: remove concrete files and update data dir
				if common.FileExists(ticketFilePath) {
					// Move concrete file back to data dir before deleting
					ticketDir := filepath.Join(cfgMgr.GetContextsPath(), "_tickets", ticketID)
					dataDirTicketFile := filepath.Join(ticketDir, ticketID+".md")
					dataDirSessionsFile := filepath.Join(ticketDir, "SESSIONS.md")

					// Remove data dir symlinks first
					common.RemoveSymlink(dataDirTicketFile)
					common.RemoveSymlink(dataDirSessionsFile)

					// Move concrete files to data dir
					if err := common.CopyFile(ticketFilePath, dataDirTicketFile); err != nil {
						warningMsg(fmt.Sprintf("Failed to move ticket file to data dir: %v", err))
					} else {
						os.Remove(ticketFilePath)
						successMsg("Moved ticket file to data directory")
					}

					if common.FileExists(sessionsFilePath) {
						if err := common.CopyFile(sessionsFilePath, dataDirSessionsFile); err != nil {
							warningMsg(fmt.Sprintf("Failed to move sessions file to data dir: %v", err))
						} else {
							os.Remove(sessionsFilePath)
						}
					}

					// Clear primary context
					ticket.PrimaryContextName = ""
				}
			} else {
				// Secondary project: just remove symlinks
				if common.FileExists(ticketFilePath) {
					if err := common.RemoveSymlink(ticketFilePath); err != nil {
						warningMsg(fmt.Sprintf("Failed to remove symlink: %v", err))
					} else {
						successMsg("Removed symlink")
					}
				}

				if common.FileExists(sessionsFilePath) {
					common.RemoveSymlink(sessionsFilePath)
				}
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
	Use:   "show [<ticket-id>]",
	Short: "Show detailed ticket information",
	Long: `Show detailed information about a specific ticket.

Example:
  cctx ticket show TICKET-123
  cctx ticket show              # ticket ID from the current branch`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTicketShow,
}

func runTicketShow(cmd *cobra.Command, args []string) error {
	// Get data directory
	dataDir := GetDataDirOrExit()

	// Load config
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	var ticketID string
	if len(args) > 0 {
		ticketID = args[0]
	} else if ticketID = resolveTicketID(cfg); ticketID == "" {
		return fmt.Errorf("no ticket specified. Pass a ticket ID, use --ticket/-t, set CCTX_TICKET, or checkout a feature branch")
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
	if ticket.WorkspacePath != "" {
		fmt.Printf("Workspace: %s\n", canon(ticket.WorkspacePath))
		if common.DirExists(ticket.WorkspacePath) {
			listWorkspaceFiles(ticket.WorkspacePath)
		} else {
			fmt.Println("  (missing)")
		}
	} else {
		fmt.Printf("Linked Projects: %d\n", len(ticket.LinkedProjects))
		for _, lp := range ticket.LinkedProjects {
			fmt.Printf("  - %s (%s)\n", lp.ContextName, lp.ProjectPath)
		}
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
	Short: "Mark ticket as completed and archive its workspace",
	Long: `Mark a ticket as completed and archive its .cctx/<ID>/ workspace.

This is the same as 'cctx archive <ID>' without the confirmation prompt.

Git info is detected from the project directory that holds the workspace:
  - Branch name: stored in ticket notes
  - Latest commit: stored in ticket commits
  - Override with --commits flag if needed

Examples:
  cctx ticket complete
  cctx ticket complete TICKET-123
  cctx ticket complete TICKET-123 --commits "abc123,def456" --prs "42,43"`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTicketComplete,
}

func runTicketComplete(cmd *cobra.Command, args []string) error {
	dataDir := GetDataDirOrExit()
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	var ticketID string
	if len(args) > 0 {
		ticketID = args[0]
	} else {
		ticketID = resolveTicketID(cfg)
		if ticketID == "" {
			return fmt.Errorf("cannot auto-detect ticket ID: not on a feature branch")
		}
		infoMsg(fmt.Sprintf("Auto-detected ticket ID: %s", ticketID))
	}

	ticket := cfg.GetTicket(ticketID, false)
	if ticket == nil {
		// Already archived or deleted, but possibly still in the working list
		if cfg.IsCurrentlyWorking(ticketID) {
			return dropStaleWorkingEntry(cfgMgr, cfg, ticketID)
		}
		return fmt.Errorf("ticket not found: %s", ticketID)
	}

	if ticket.Status == "completed" {
		if err := dropStaleWorkingEntry(cfgMgr, cfg, ticketID); err != nil {
			return err
		}
		return fmt.Errorf("ticket is already completed")
	}

	return archiveTicket(cfgMgr, ticketID, ticketArchiveOpts{
		approveCompletion: true,
		completeAbandoned: true,
		commits:           ticketCommits,
		prs:               ticketPRs,
	})
}

// ticket abandon subcommand
var ticketAbandonCmd = &cobra.Command{
	Use:   "abandon <ticket-id>",
	Short: "Mark ticket as abandoned",
	Long: `Mark a ticket as abandoned with optional reason.

Example:
  cctx ticket abandon
  cctx ticket abandon TICKET-123
  cctx ticket abandon TICKET-123 --reason "Requirements changed"`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTicketAbandon,
}

func runTicketAbandon(cmd *cobra.Command, args []string) error {
	var ticketID string
	if len(args) > 0 {
		ticketID = args[0]
	} else {
		branch := common.GetGitBranch()
		if branch == "" || branch == "main" || branch == "master" {
			return fmt.Errorf("cannot auto-detect ticket ID: not on a feature branch (current: %s)", branch)
		}
		ticketID = branch
		infoMsg(fmt.Sprintf("Auto-detected ticket ID from branch: %s", ticketID))
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
	infoMsg("You can archive it with: cctx archive " + ticketID)

	// Remove from working tickets if present
	if cfg.IsCurrentlyWorking(ticketID) {
		cfg.RemoveCurrentWorkingTicket(ticketID)
		if err := cfgMgr.Save(cfg); err != nil {
			warningMsg(fmt.Sprintf("Failed to remove from working tickets: %v", err))
		}
	}

	return nil
}

// ticket restore subcommand
var ticketRestoreCmd = &cobra.Command{
	Use:   "restore <ticket-id>",
	Short: "Restore an archived ticket for active development",
	Long: `Restore an archived ticket back to active status.

A ticket created with directory workspaces is copied back into
./.cctx/<ticket-id>/ in the current directory (wherever it was archived from),
verified, and its archive removed. An existing workspace is never overwritten.

Older single-file tickets are restored as before: their files are copied into
the current directory and the data directory symlinks are recreated.

Examples:
  cctx ticket restore CBP-123
  cctx ticket restore            # ticket ID from the current branch`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTicketRestore,
}

func runTicketRestore(cmd *cobra.Command, args []string) error {
	var ticketID string
	dataDir := GetDataDirOrExit()

	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if len(args) > 0 {
		ticketID = args[0]
	} else {
		ticketID = resolveTicketID(cfg)
		if ticketID == "" {
			return fmt.Errorf("no ticket specified. Pass a ticket ID, use --ticket/-t, set CCTX_TICKET, or checkout a feature branch")
		}
	}
	ticketID = strings.TrimSuffix(ticketID, ".md")

	ticket := cfg.GetTicket(ticketID, true)
	if ticket == nil {
		return fmt.Errorf("ticket not found: %s", ticketID)
	}
	if ticket.ArchivedPath == "" || cfg.GetTicket(ticketID, false) != nil {
		return fmt.Errorf("ticket %s is not archived (status: %s)", ticketID, ticket.Status)
	}
	if ticket.WorkspacePath != "" {
		return restoreDirectoryTicket(cfgMgr, ticket)
	}
	if !validTicketID(ticketID) {
		return fmt.Errorf("invalid ticket ID %q", ticketID)
	}

	archivedDir, ok := safeArchiveDir(cfgMgr, ticket.ArchivedPath)
	if !ok {
		return fmt.Errorf("refusing suspicious archived_path %q", ticket.ArchivedPath)
	}
	if !common.DirExists(archivedDir) {
		return fmt.Errorf("archived directory not found: %s", archivedDir)
	}

	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// FindTicketFiles deliberately excludes SESSIONS.md since it doesn't
	// contain the ticket ID, so it's added separately here.
	filesToRestore := FindTicketFiles(archivedDir, ticketID)
	sessionsFile := filepath.Join(archivedDir, "SESSIONS.md")
	if common.FileExists(sessionsFile) {
		filesToRestore = append(filesToRestore, sessionsFile)
	}

	if len(filesToRestore) == 0 {
		return fmt.Errorf("no files found in archive: %s", archivedDir)
	}

	// Refuse to clobber files already sitting in the current directory.
	for _, src := range filesToRestore {
		dest := filepath.Join(currentDir, filepath.Base(src))
		if common.FileExists(dest) {
			return fmt.Errorf("refusing to overwrite existing file: %s", dest)
		}
	}

	fmt.Println()
	infoMsg(fmt.Sprintf("Restoring ticket: %s", ticketID))
	infoMsg(fmt.Sprintf("From: %s", archivedDir))
	infoMsg(fmt.Sprintf("To: %s", currentDir))
	fmt.Println()
	for _, src := range filesToRestore {
		fmt.Printf("  - %s\n", filepath.Base(src))
	}
	fmt.Println()

	if dryRun {
		dryRunMsg(fmt.Sprintf("Would copy %d file(s) to %s", len(filesToRestore), currentDir))
		dryRunMsg("Would recreate data directory symlinks")
		dryRunMsg("Would move ticket from archived to active")
		return nil
	}

	if !ticketForce && !common.Confirm("Proceed with restore?", true) {
		infoMsg("Operation cancelled")
		return nil
	}
	fmt.Println()

	for _, src := range filesToRestore {
		dest := filepath.Join(currentDir, filepath.Base(src))
		if err := common.CopyFile(src, dest); err != nil {
			return fmt.Errorf("failed to restore %s: %w", filepath.Base(src), err)
		}
		os.Remove(src)
		successMsg(fmt.Sprintf("Restored %s", filepath.Base(src)))
	}
	os.Remove(archivedDir) // best-effort; no-op if anything unexpected remains

	// Recreate the data directory symlinks for the ticket file and SESSIONS.md
	ticketDir := filepath.Join(cfgMgr.GetContextsPath(), "_tickets", ticketID)
	if err := common.EnsureDir(ticketDir); err != nil {
		return fmt.Errorf("failed to create ticket directory: %w", err)
	}

	concreteTicketFile := filepath.Join(currentDir, ticketID+".md")
	if err := common.CreateSymlink(concreteTicketFile, filepath.Join(ticketDir, ticketID+".md")); err != nil {
		warningMsg(fmt.Sprintf("Failed to create data dir symlink: %v", err))
	}

	concreteSessionsFile := filepath.Join(currentDir, "SESSIONS.md")
	if common.FileExists(concreteSessionsFile) {
		if err := common.CreateSymlink(concreteSessionsFile, filepath.Join(ticketDir, "SESSIONS.md")); err != nil {
			warningMsg(fmt.Sprintf("Failed to create data dir sessions symlink: %v", err))
		}
	}
	successMsg("Recreated data directory symlinks")

	// Reopen the ticket against whatever project the current directory resolves to
	projectName, _ := GetProjectContext(dataDir)
	var linkedProjects []config.LinkedProject
	if projectName != "" {
		if project := cfg.GetProject(projectName); project != nil {
			linkedProjects = append(linkedProjects, config.LinkedProject{ContextName: projectName, ProjectPath: project.ProjectPath})
		}
	}
	if len(linkedProjects) == 0 {
		linkedProjects = append(linkedProjects, config.LinkedProject{ContextName: filepath.Base(currentDir), ProjectPath: currentDir})
	}

	// Drop the archive's record once it's emptied; otherwise keep it, unowned
	var records []config.ArchiveRecord
	for _, r := range cfg.Archives {
		if filepath.Clean(r.ArchiveDir) == filepath.Clean(ticket.ArchivedPath) {
			if !common.DirExists(archivedDir) {
				continue
			}
			r.FileCount = countArchiveEntries(archivedDir)
			r.TicketID = ""
		}
		records = append(records, r)
	}
	cfg.Archives = records

	ticket.Status = "active"
	ticket.ArchivedPath = ""
	ticket.CompletedAt = nil
	ticket.AbandonedAt = nil
	ticket.LinkedProjects = linkedProjects
	ticket.PrimaryContextName = linkedProjects[0].ContextName
	ticket.LastModified = time.Now()

	newArchived := []config.Ticket{}
	for _, t := range cfg.Tickets.Archived {
		if t.TicketID != ticketID {
			newArchived = append(newArchived, t)
		}
	}
	cfg.Tickets.Archived = newArchived
	cfg.Tickets.Active = append(cfg.Tickets.Active, *ticket)

	cfg.AddCurrentWorkingTicket(ticketID, ticket.PrimaryContextName)

	if err := cfgMgr.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	successMsg("Updated configuration")

	fmt.Println()
	successMsg(fmt.Sprintf("Successfully restored ticket: %s", ticketID))
	infoMsg(fmt.Sprintf("Ticket file: %s", concreteTicketFile))
	infoMsg(fmt.Sprintf("Linked to: %s", ticket.PrimaryContextName))

	return nil
}

// ticket edit subcommand
var ticketEditCmd = &cobra.Command{
	Use:   "edit <ticket-id>",
	Short: "Edit ticket metadata",
	Long: `Edit ticket metadata (title, tags, notes).

Example:
  cctx ticket edit TICKET-123 --title "New title"
  cctx ticket edit TICKET-123 --tags "bug,urgent" --notes "Updated requirements"`,
	Args: cobra.ExactArgs(1),
	RunE: runTicketEdit,
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

This action cannot be undone.

Example:
  cctx ticket delete TICKET-123
  cctx ticket delete TICKET-123 --force  # Skip confirmation`,
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
	if ticket != nil && ticket.WorkspacePath != "" {
		return deleteDirectoryTicket(cfgMgr, cfg, ticket)
	}

	// Determine ticket directory
	var ticketDir string
	archiveRemoved := false
	if ticket != nil {
		if ticket.ArchivedPath != "" {
			if archived, ok := safeArchiveDir(cfgMgr, ticket.ArchivedPath); ok {
				ticketDir = archived
			} else {
				warningMsg(fmt.Sprintf("Not removing suspicious archived_path %q", ticket.ArchivedPath))
			}
		} else {
			ticketDir = filepath.Join(cfgMgr.GetContextsPath(), "_tickets", ticketID)
		}
	} else {
		ticketDir = filepath.Join(cfgMgr.GetContextsPath(), "_tickets", ticketID)
	}
	// An ID like ../_archived must never resolve outside _tickets/
	if ticketDir != "" && (ticket == nil || ticket.ArchivedPath == "") {
		ticketsRoot := canon(filepath.Join(cfgMgr.GetContextsPath(), "_tickets"))
		if filepath.Dir(canon(ticketDir)) != ticketsRoot {
			warningMsg(fmt.Sprintf("Not removing %s: it is outside %s", ticketDir, ticketsRoot))
			ticketDir = ""
		}
	}

	// Check if there are any orphaned files to clean up
	hasOrphanedFiles := false
	if cwd, err := os.Getwd(); err == nil {
		symlinkPath := filepath.Join(cwd, ticketID+".md")
		sessionsSymlinkPath := filepath.Join(cwd, "SESSIONS.md")
		if common.FileExists(symlinkPath) || common.FileExists(sessionsSymlinkPath) {
			hasOrphanedFiles = true
		}
	}
	if !hasOrphanedFiles && common.DirExists(ticketDir) {
		hasOrphanedFiles = true
	}

	// If ticket not found in config and no orphaned files, exit
	if ticket == nil && !hasOrphanedFiles {
		return fmt.Errorf("ticket not found: %s", ticketID)
	}

	fmt.Println()
	if ticket != nil {
		warningMsg(fmt.Sprintf("About to permanently delete ticket: %s", ticketID))
		if ticket.Title != "" {
			infoMsg(fmt.Sprintf("Title: %s", ticket.Title))
		}
		infoMsg(fmt.Sprintf("Status: %s", ticket.Status))
		infoMsg(fmt.Sprintf("Linked projects: %d", len(ticket.LinkedProjects)))
	} else {
		warningMsg(fmt.Sprintf("Cleaning up orphaned files for: %s", ticketID))
		warningMsg("(Ticket not found in config)")
	}
	fmt.Println()
	warningMsg("This will remove:")
	if ticket != nil {
		warningMsg("  - Ticket from config.json")
	}
	warningMsg("  - Ticket directory and all files")
	warningMsg("  - Symlinks from all linked projects")
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

	// Remove symlinks from ALL managed projects (not just LinkedProjects)
	// This handles cases where symlinks exist but weren't tracked in LinkedProjects
	for _, project := range cfg.ManagedProjects {
		symlinkPath := filepath.Join(project.ProjectPath, ticketID+".md")
		sessionsSymlinkPath := filepath.Join(project.ProjectPath, "SESSIONS.md")

		// Check if ticket symlink exists
		ticketSymlinkExists := common.FileExists(symlinkPath)
		_, lerr := os.Lstat(sessionsSymlinkPath)
		sessionsSymlinkExists := lerr == nil

		if !ticketSymlinkExists && !sessionsSymlinkExists {
			continue
		}

		if dryRun {
			if ticketSymlinkExists {
				dryRunMsg(fmt.Sprintf("Would remove ticket file from %s", project.ContextName))
			}
			if sessionsSymlinkExists {
				dryRunMsg(fmt.Sprintf("Would remove SESSIONS.md file from %s", project.ContextName))
			}
		} else {
			// Remove ticket file (symlink or concrete)
			if ticketSymlinkExists {
				var err error
				if common.IsSymlink(symlinkPath) {
					err = common.RemoveSymlink(symlinkPath)
				} else {
					err = os.Remove(symlinkPath)
				}
				if err != nil {
					warningMsg(fmt.Sprintf("Failed to remove ticket file from %s: %v", project.ContextName, err))
				} else {
					successMsg(fmt.Sprintf("Removed ticket file from %s", project.ContextName))
				}
			}

			// A concrete SESSIONS.md may belong to another ticket, so only the
			// symlink into this ticket's data dir is removed
			if sessionsSymlinkExists && !isTicketSessionsLink(sessionsSymlinkPath, ticketID) {
				warningMsg(fmt.Sprintf("Left SESSIONS.md in %s (not linked to %s)", project.ContextName, ticketID))
			} else if sessionsSymlinkExists {
				if err := common.RemoveSymlink(sessionsSymlinkPath); err != nil {
					warningMsg(fmt.Sprintf("Failed to remove SESSIONS.md file from %s: %v", project.ContextName, err))
				} else {
					successMsg(fmt.Sprintf("Removed SESSIONS.md file from %s", project.ContextName))
				}
			}
		}
	}

	// Also check current directory for orphaned files (from unmanaged directories)
	if cwd, err := os.Getwd(); err == nil {
		symlinkPath := filepath.Join(cwd, ticketID+".md")
		sessionsSymlinkPath := filepath.Join(cwd, "SESSIONS.md")

		if common.FileExists(symlinkPath) {
			if dryRun {
				dryRunMsg(fmt.Sprintf("Would remove file from current directory: %s", symlinkPath))
			} else {
				var err error
				if common.IsSymlink(symlinkPath) {
					err = common.RemoveSymlink(symlinkPath)
				} else {
					err = os.Remove(symlinkPath)
				}
				if err != nil {
					warningMsg(fmt.Sprintf("Failed to remove file from current directory: %v", err))
				} else {
					successMsg("Removed file from current directory")
				}
			}
		}

		// Also check for SESSIONS.md file
		_, sessionsErr := os.Lstat(sessionsSymlinkPath)
		if sessionsErr == nil && !isTicketSessionsLink(sessionsSymlinkPath, ticketID) {
			warningMsg(fmt.Sprintf("Left SESSIONS.md in current directory (not linked to %s)", ticketID))
		} else if sessionsErr == nil {
			if dryRun {
				dryRunMsg(fmt.Sprintf("Would remove SESSIONS.md file from current directory"))
			} else {
				if err := common.RemoveSymlink(sessionsSymlinkPath); err != nil {
					warningMsg(fmt.Sprintf("Failed to remove SESSIONS.md file: %v", err))
				} else {
					successMsg("Removed SESSIONS.md file from current directory")
				}
			}
		}
	}

	// Remove ticket directory
	if dryRun {
		dryRunMsg(fmt.Sprintf("Would delete directory: %s", ticketDir))
	} else {
		if ticketDir != "" && common.DirExists(ticketDir) {
			if err := os.RemoveAll(ticketDir); err != nil {
				return fmt.Errorf("failed to delete ticket directory: %w", err)
			}
			archiveRemoved = ticket != nil && ticket.ArchivedPath != ""
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

	if cfg.IsCurrentlyWorking(ticketID) {
		cfg.RemoveCurrentWorkingTicket(ticketID)
	}

	// The ticket's own archive goes; archives made by glob/path that merely
	// carry its ID stay listed
	var records []config.ArchiveRecord
	for _, r := range cfg.Archives {
		if ticket != nil && ticket.ArchivedPath != "" && filepath.Clean(r.ArchiveDir) == filepath.Clean(ticket.ArchivedPath) &&
			(archiveRemoved || !common.DirExists(filepath.Join(dataDir, r.ArchiveDir))) {
			continue
		}
		if r.TicketID == ticketID {
			r.TicketID = ""
		}
		records = append(records, r)
	}
	cfg.Archives = records

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

// isTicketSessionsLink reports whether path is a symlink into ticketID's data dir.
func isTicketSessionsLink(path, ticketID string) bool {
	if !common.IsSymlink(path) {
		return false
	}
	target, err := os.Readlink(path)
	return err == nil && strings.Contains(target, string(filepath.Separator)+filepath.Join("_tickets", ticketID)+string(filepath.Separator))
}

// workingTicketIsStale reports whether a working-list entry points at a ticket
// that can no longer be worked on: archived, finished, or gone from config.
func workingTicketIsStale(cfg *config.Config, ticketID string) bool {
	ticket := cfg.GetTicket(ticketID, false)
	return ticket == nil || ticket.Status == "completed" || ticket.Status == "abandoned"
}

// dropStaleWorkingEntry removes a stale working-list entry and saves the config.
// Without this, completing such a ticket is impossible: every completion path
// looks the ticket up in active tickets first, so the entry resurfaces forever.
func dropStaleWorkingEntry(cfgMgr *config.Manager, cfg *config.Config, ticketID string) error {
	if !cfg.IsCurrentlyWorking(ticketID) {
		return nil
	}
	if dryRun {
		dryRunMsg(fmt.Sprintf("Would remove stale working ticket: %s", ticketID))
		return nil
	}

	cfg.RemoveCurrentWorkingTicket(ticketID)
	if err := cfgMgr.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	successMsg(fmt.Sprintf("Removed stale working ticket: %s (no longer active)", ticketID))
	return nil
}

// pruneStaleWorkingTickets drops every stale entry from the working list and
// returns the removed ticket IDs. The caller saves the config.
func pruneStaleWorkingTickets(cfg *config.Config) []string {
	var stale []string
	for _, cwt := range cfg.CurrentWorkingTickets {
		if workingTicketIsStale(cfg, cwt.TicketID) {
			stale = append(stale, cwt.TicketID)
		}
	}
	for _, ticketID := range stale {
		cfg.RemoveCurrentWorkingTicket(ticketID)
	}
	return stale
}

// ticket working subcommand
var ticketWorkingCmd = &cobra.Command{
	Use:   "working [remove <ticket-id>]",
	Short: "Manage currently working tickets",
	Long: `View and manage tickets you're currently working on across all repos.

This tracks tickets you have in progress simultaneously.

Usage:
  - List working tickets: cctx ticket working
  - Remove a ticket: cctx ticket working remove TICKET-123
  - Clear all: cctx ticket working clear`,
	Args: cobra.MaximumNArgs(2),
	RunE: runTicketWorking,
}

func runTicketWorking(cmd *cobra.Command, args []string) error {
	// Get data directory
	dataDir := GetDataDirOrExit()

	// Load config
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Handle subcommands
	if len(args) > 0 {
		switch args[0] {
		case "remove", "rm":
			if len(args) < 2 {
				return fmt.Errorf("specify ticket ID to remove: cctx ticket working remove TICKET-123")
			}
			ticketID := args[1]

			if !cfg.IsCurrentlyWorking(ticketID) {
				return fmt.Errorf("ticket %s is not in working list", ticketID)
			}

			cfg.RemoveCurrentWorkingTicket(ticketID)
			if err := cfgMgr.Save(cfg); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			successMsg(fmt.Sprintf("Removed %s from working tickets", ticketID))
			return nil

		case "clear":
			if len(cfg.CurrentWorkingTickets) == 0 {
				infoMsg("No working tickets to clear")
				return nil
			}

			count := len(cfg.CurrentWorkingTickets)
			cfg.CurrentWorkingTickets = []config.CurrentTicket{}

			if err := cfgMgr.Save(cfg); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			successMsg(fmt.Sprintf("Cleared %d working ticket(s)", count))
			return nil

		default:
			return fmt.Errorf("unknown subcommand: %s (use 'remove' or 'clear')", args[0])
		}
	}

	// List working tickets
	if len(cfg.CurrentWorkingTickets) == 0 {
		infoMsg("No tickets currently being worked on")
		fmt.Println()
		infoMsg("Tickets are automatically added when you create them.")
		infoMsg("Remove completed tickets with: cctx ticket working remove <ticket-id>")
		return nil
	}

	fmt.Println()
	fmt.Println("Currently Working Tickets:")
	fmt.Println("=" + strings.Repeat("=", 60))
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "TICKET ID\tPROJECT\tSTARTED\tSTATUS\n")
	fmt.Fprintf(w, "---------\t-------\t-------\t------\n")

	for _, cwt := range cfg.CurrentWorkingTickets {
		// Get ticket details
		ticket := cfg.GetTicket(cwt.TicketID, false)
		status := "unknown"
		if ticket != nil {
			status = ticket.Status
		}

		projectName := cwt.ProjectName
		if projectName == "" {
			projectName = "-"
		}

		startedTime := cwt.StartedAt.Format("2006-01-02 15:04")

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", cwt.TicketID, projectName, startedTime, status)
	}
	w.Flush()

	fmt.Println()
	infoMsg("Commands:")
	infoMsg("  Remove: cctx ticket working remove <ticket-id>")
	infoMsg("  Clear all: cctx ticket working clear")
	fmt.Println()

	return nil
}
