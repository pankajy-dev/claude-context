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
	"github.com/pankaj/claude-context/internal/templates"
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

	// ticket archive
	ticketCmd.AddCommand(ticketArchiveCmd)
	ticketArchiveCmd.InheritedFlags().MarkHidden("ticket")

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

	// ticket archive-all
	ticketCmd.AddCommand(ticketArchiveAllCmd)
	ticketArchiveAllCmd.Flags().BoolVarP(&ticketForce, "force", "f", false, "Skip confirmation prompt")
	ticketArchiveAllCmd.InheritedFlags().MarkHidden("ticket")

	// ticket working
	ticketCmd.AddCommand(ticketWorkingCmd)
	ticketWorkingCmd.InheritedFlags().MarkHidden("ticket")
}

func runTicketCreate(cmd *cobra.Command, args []string) error {
	var ticketID string
	var linkToExisting bool

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
				fmt.Println("  1. Yes, mark one as completed")
				fmt.Println("  2. No, continue with multiple active tickets")
				fmt.Println("  3. Cancel")
				fmt.Print("\nEnter choice [1-3]: ")

				var choice string
				fmt.Scanln(&choice)
				fmt.Println()

				switch choice {
				case "1":
					// Prompt for which ticket to complete
					fmt.Print("Enter ticket ID to complete: ")
					var ticketToComplete string
					fmt.Scanln(&ticketToComplete)
					fmt.Println()

					// Mark the ticket as completed
					oldTicket := cfg.GetTicket(ticketToComplete, false)
					if oldTicket != nil && oldTicket.Status != "completed" {
						completedAt := time.Now()
						oldTicket.Status = "completed"
						oldTicket.CompletedAt = &completedAt
						oldTicket.LastModified = time.Now()

						// Remove from working list
						cfg.RemoveCurrentWorkingTicket(ticketToComplete)

						// Save the config
						if err := cfgMgr.Save(cfg); err != nil {
							return fmt.Errorf("failed to save config: %w", err)
						}
						successMsg(fmt.Sprintf("Marked ticket %s as completed", ticketToComplete))
						fmt.Println()
					} else if oldTicket == nil {
						warningMsg(fmt.Sprintf("Ticket %s not found", ticketToComplete))
					} else {
						warningMsg(fmt.Sprintf("Ticket %s already completed", ticketToComplete))
					}
				case "2":
					infoMsg("Continuing with multiple active tickets")
					fmt.Println()
				case "3", "":
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
		// Explicit ticket ID provided
		ticketID = args[0]
		existingTicket := cfg.GetTicket(ticketID, true)

		if existingTicket != nil {
			// Check if ticket is already linked to current project
			isLinkedToCurrent := false
			if currentProject != "" {
				for _, lp := range existingTicket.LinkedProjects {
					if lp.ContextName == currentProject {
						isLinkedToCurrent = true
						break
					}
				}
			}

			if isLinkedToCurrent {
				return fmt.Errorf("ticket %s already exists and is linked to project: %s", ticketID, currentProject)
			}

			// Ticket exists but not linked to current project
			fmt.Println()
			warningMsg(fmt.Sprintf("Ticket '%s' already exists", ticketID))
			if len(existingTicket.LinkedProjects) > 0 {
				projectNames := []string{}
				for _, lp := range existingTicket.LinkedProjects {
					projectNames = append(projectNames, lp.ContextName)
				}
				infoMsg(fmt.Sprintf("Linked to: %s", strings.Join(projectNames, ", ")))
			}
			if existingTicket.Title != "" {
				infoMsg(fmt.Sprintf("Title: %s", existingTicket.Title))
			}
			fmt.Println()

			// Ask user what to do (unless in dry-run mode)
			if dryRun {
				warningMsg("In dry-run mode, would prompt user for action")
			} else {
				// Prepare option for project-scoped ticket
				projectScopedID := ""
				if currentProject != "" {
					projectScopedID = fmt.Sprintf("%s-%s", currentProject, ticketID)
				}

				// Show options
				fmt.Println("Choose an option:")
				fmt.Println("  1. Link to existing ticket (share across projects)")
				if projectScopedID != "" && cfg.GetTicket(projectScopedID, true) == nil {
					fmt.Printf("  2. Create project-scoped ticket: %s\n", projectScopedID)
				}
				fmt.Println("  3. Create with auto-incremented suffix (e.g., main-1)")
				fmt.Println("  4. Cancel")
				fmt.Print("\nEnter choice [1-4]: ")

				var choice string
				fmt.Scanln(&choice)
				fmt.Println()

				switch choice {
				case "1":
					linkToExisting = true
					infoMsg(fmt.Sprintf("Linking to existing ticket: %s", ticketID))
				case "2":
					if projectScopedID != "" && cfg.GetTicket(projectScopedID, true) == nil {
						ticketID = projectScopedID
						infoMsg(fmt.Sprintf("Creating project-scoped ticket: %s", ticketID))
					} else {
						return fmt.Errorf("invalid choice or project-scoped ticket already exists")
					}
				case "3":
					originalID := ticketID
					suffix := 1
					for cfg.GetTicket(ticketID, true) != nil {
						ticketID = fmt.Sprintf("%s-%d", originalID, suffix)
						suffix++
					}
					infoMsg(fmt.Sprintf("Creating ticket with suffix: %s", ticketID))
				case "4", "":
					infoMsg("Operation cancelled")
					return nil
				default:
					return fmt.Errorf("invalid choice: %s", choice)
				}
			}
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

	// Create or update ticket metadata
	if linkToExisting {
		// Link existing ticket to current project
		existingTicket := cfg.GetTicket(ticketID, true)
		if existingTicket != nil && projectName != "" {
			// Check if project is already linked
			alreadyLinked := false
			for _, lp := range existingTicket.LinkedProjects {
				if lp.ContextName == projectName {
					alreadyLinked = true
					break
				}
			}

			if !alreadyLinked {
				// Add current project to linked projects
				existingTicket.LinkedProjects = append(existingTicket.LinkedProjects, linkedProjects...)
				existingTicket.LastModified = time.Now()

				if !dryRun {
					if err := cfgMgr.Save(cfg); err != nil {
						return fmt.Errorf("failed to save config: %w", err)
					}
					successMsg("Linked ticket to project")
				}
			}
		}
	} else {
		// Create new ticket metadata with v2 architecture
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

		// Set primary context (first linked project)
		if len(linkedProjects) > 0 {
			ticket.PrimaryContextName = linkedProjects[0].ContextName
		}

		// Add to config
		if !dryRun {
			cfg.Tickets.Active = append(cfg.Tickets.Active, ticket)
			if err := cfgMgr.Save(cfg); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}
			successMsg("Updated configuration")
		}
	}

	// V2 Architecture: ALWAYS create concrete files in current directory
	// Get current directory first
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Concrete files always go in current directory (v2 behavior)
	ticketDir := filepath.Join(cfgMgr.GetContextsPath(), "_tickets", ticketID)
	concreteTicketFile := filepath.Join(currentDir, ticketID+".md")
	concreteSessionsFile := filepath.Join(currentDir, "SESSIONS.md")

	// Create concrete files (skip if linking to existing)
	if !linkToExisting {
		if dryRun {
			dryRunMsg(fmt.Sprintf("Would create ticket directory: %s", ticketDir))
			dryRunMsg(fmt.Sprintf("Would create concrete ticket file: %s", concreteTicketFile))
			dryRunMsg(fmt.Sprintf("Would create concrete sessions file: %s", concreteSessionsFile))
		} else {
			// Ensure ticket directory in data dir always exists (for symlinks)
			if err := common.EnsureDir(ticketDir); err != nil {
				return fmt.Errorf("failed to create ticket directory: %w", err)
			}

			// Create ticket.md from template
			ticketContent, _, err := templates.GetTemplate("ticket", dataDir)
			if err != nil {
				return fmt.Errorf("failed to load ticket template: %w", err)
			}

			// Replace {{TICKET_ID}} placeholder with actual ticket ID
			content := strings.ReplaceAll(string(ticketContent), "{{TICKET_ID}}", ticketID)

			// Prepend ticket header
			fullContent := fmt.Sprintf("# Ticket: %s\n\n", ticketID) + content

			if err := os.WriteFile(concreteTicketFile, []byte(fullContent), 0644); err != nil {
				return fmt.Errorf("failed to create ticket file: %w", err)
			}
			successMsg(fmt.Sprintf("Created ticket file: %s", concreteTicketFile))

			// Create SESSIONS.md from template
			sessionsContent, _, err := templates.GetTemplate("sessions", dataDir)
			if err != nil {
				return fmt.Errorf("failed to load sessions template: %w", err)
			}

			if err := os.WriteFile(concreteSessionsFile, sessionsContent, 0644); err != nil {
				warningMsg(fmt.Sprintf("Failed to create SESSIONS.md: %v", err))
			} else {
				successMsg(fmt.Sprintf("Created SESSIONS.md: %s", concreteSessionsFile))
			}

			// Create symlinks in data dir pointing to concrete files (if not already in data dir)
			dataDirTicketFile := filepath.Join(ticketDir, ticketID+".md")
			dataDirSessionsFile := filepath.Join(ticketDir, "SESSIONS.md")

			if concreteTicketFile != dataDirTicketFile {
				if err := common.CreateSymlink(concreteTicketFile, dataDirTicketFile); err != nil {
					warningMsg(fmt.Sprintf("Failed to create data dir symlink: %v", err))
				} else {
					successMsg("Created symlink in data directory")
				}
			}

			if concreteSessionsFile != dataDirSessionsFile {
				if err := common.CreateSymlink(concreteSessionsFile, dataDirSessionsFile); err != nil {
					warningMsg(fmt.Sprintf("Failed to create data dir sessions symlink: %v", err))
				}
			}
		}
	} else {
		// Linking to existing ticket - create symlinks to primary project's concrete files
		if dryRun {
			dryRunMsg(fmt.Sprintf("Would create symlink: %s", concreteTicketFile))
		} else {
			existingTicket := cfg.GetTicket(ticketID, true)
			if existingTicket == nil {
				return fmt.Errorf("existing ticket not found: %s", ticketID)
			}

			// Determine target based on primary project
			var ticketTarget, sessionsTarget string
			if existingTicket.PrimaryContextName != "" {
				primaryProject := cfg.GetProject(existingTicket.PrimaryContextName)
				if primaryProject != nil && common.FileExists(filepath.Join(primaryProject.ProjectPath, ticketID+".md")) {
					ticketTarget = filepath.Join(primaryProject.ProjectPath, ticketID+".md")
					sessionsTarget = filepath.Join(primaryProject.ProjectPath, "SESSIONS.md")
				}
			}

			// Fallback to data dir if primary not accessible
			if ticketTarget == "" {
				ticketFile := filepath.Join(cfgMgr.GetContextsPath(), "_tickets", ticketID, ticketID+".md")
				if common.FileExists(ticketFile) {
					ticketTarget = ticketFile
					sessionsTarget = filepath.Join(cfgMgr.GetContextsPath(), "_tickets", ticketID, "SESSIONS.md")
				}
			}

			if ticketTarget == "" {
				return fmt.Errorf("could not find ticket files for: %s", ticketID)
			}

			// Create symlink to ticket file
			if err := common.CreateSymlink(ticketTarget, concreteTicketFile); err != nil {
				return fmt.Errorf("failed to create symlink: %w", err)
			}
			successMsg(fmt.Sprintf("Created symlink: %s", concreteTicketFile))

			// Create symlink to sessions file if it exists
			if common.FileExists(sessionsTarget) {
				if err := common.CreateSymlink(sessionsTarget, concreteSessionsFile); err != nil {
					warningMsg(fmt.Sprintf("Failed to create sessions symlink: %v", err))
				} else {
					successMsg(fmt.Sprintf("Created sessions symlink: %s", concreteSessionsFile))
				}
			}
		}
	}

	// V2: Concrete files already created in current directory
	// Just update .clauderc (ticket.md only, not SESSIONS.md)
	if !dryRun {
		rcMgr := clauderc.NewManager(currentDir)
		ticketFileName := ticketID + ".md"
		if err := rcMgr.AddFile(ticketFileName, dryRun); err != nil {
			warningMsg(fmt.Sprintf("Failed to add %s to .clauderc: %v", ticketFileName, err))
		} else {
			successMsg(fmt.Sprintf("Added %s to .clauderc", ticketFileName))
		}
	}

	// Git commit removed (no longer tracking in git)

	if !dryRun {
		fmt.Println()
		if linkToExisting {
			successMsg(fmt.Sprintf("Successfully linked to existing ticket: %s", ticketID))
		} else {
			successMsg(fmt.Sprintf("Successfully created ticket: %s", ticketID))
			if ticketTitle != "" {
				infoMsg(fmt.Sprintf("Title: %s", ticketTitle))
			}
		}
		infoMsg(fmt.Sprintf("Location: %s", ticketDir))
		infoMsg(fmt.Sprintf("Ticket file: %s", concreteTicketFile))
		if projectName != "" {
			infoMsg(fmt.Sprintf("Linked to project: %s", projectName))
		}
		fmt.Println()
		infoMsg("Next steps:")
		infoMsg(fmt.Sprintf("  1. Edit ticket context: vim %s.md", ticketID))
		if projectName == "" {
			infoMsg(fmt.Sprintf("  2. Link to a project: cctx ticket link %s <project>", ticketID))
		} else {
			infoMsg(fmt.Sprintf("  2. (Optional) Link to other projects: cctx -t %s ticket link <project>", ticketID))
		}

		// Add to current working tickets (cross-repo tracking)
		cfg.AddCurrentWorkingTicket(ticketID, projectName)
		if err := cfgMgr.Save(cfg); err != nil {
			warningMsg(fmt.Sprintf("Failed to add to working tickets: %v", err))
		}
	}

	return nil
}

// ticket link subcommand
var ticketLinkCmd = &cobra.Command{
	Use:   "link <ticket-id> [project...]",
	Short: "Link ticket to one or more projects",
	Long: `Link a ticket to one or more projects.

Creates symlinks in project directories and updates .clauderc automatically.

Examples:
  cctx ticket link TICKET-123 api-gateway frontend
  cctx ticket link TICKET-123 --project api-gateway
  cctx ticket link TICKET-123  # Auto-detects current project`,
	Args: cobra.MinimumNArgs(1),
	RunE: runTicketLink,
}

func runTicketLink(cmd *cobra.Command, args []string) error {
	// Get ticket ID from first argument
	ticketID := args[0]

	// Get data directory
	dataDir := GetDataDirOrExit()

	// Determine project names from remaining args
	projectNames := []string{}

	if len(args) > 1 {
		// Projects specified as additional arguments
		projectNames = args[1:]
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
			dryRunMsg(fmt.Sprintf("Would add to .clauderc"))
		} else {
			// For primary project, ensure concrete files are in project
			if ticket.PrimaryContextName == projectName {
				// Check if files already exist in primary project (new v2 tickets)
				if common.FileExists(symlinkPath) && !common.IsSymlink(symlinkPath) {
					successMsg("Concrete files already in primary project")
				} else {
					// Migrate from old location (data dir) if needed
					if common.FileExists(ticketFile) && !common.IsSymlink(ticketFile) {
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
					}

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
	Use:   "unlink <ticket-id> [project...]",
	Short: "Unlink ticket from project(s)",
	Long: `Unlink a ticket from one or more projects.

Removes symlinks and updates .clauderc automatically.

Examples:
  cctx ticket unlink TICKET-123 api-gateway frontend
  cctx ticket unlink TICKET-123 --project api-gateway
  cctx ticket unlink TICKET-123  # Auto-detects current project
  cctx ticket unlink TICKET-123 --all  # Unlink from all projects`,
	Args: cobra.MinimumNArgs(1),
	RunE: runTicketUnlink,
}

func runTicketUnlink(cmd *cobra.Command, args []string) error {
	// Get ticket ID from first argument
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

	// Determine project names from remaining args
	projectNames := []string{}

	if ticketAll {
		// --all flag: unlink from all linked projects
		for _, lp := range ticket.LinkedProjects {
			projectNames = append(projectNames, lp.ContextName)
		}
	} else if len(args) > 1 {
		// Projects specified as additional arguments
		projectNames = args[1:]
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
			dryRunMsg(fmt.Sprintf("Would remove from .clauderc"))
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
	Use:   "show <ticket-id>",
	Short: "Show detailed ticket information",
	Long: `Show detailed information about a specific ticket.

Example:
  cctx ticket show TICKET-123`,
	Args: cobra.ExactArgs(1),
	RunE: runTicketShow,
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

Examples:
  cctx ticket complete TICKET-123
  cctx ticket complete TICKET-123 --commits "abc123,def456" --prs "42,43"`,
	Args: cobra.ExactArgs(1),
	RunE: runTicketComplete,
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

	fmt.Println()
	infoMsg(fmt.Sprintf("Marking ticket %s as completed and archiving...", ticketID))

	// Auto-archive: Remove symlinks from all linked projects
	fmt.Println()
	infoMsg("Removing symlinks from projects...")

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

	// Append completion info to SESSIONS.md before archiving
	ticketDir := filepath.Join(cfgMgr.GetContextsPath(), "_tickets", ticketID)
	sessionsFile := filepath.Join(ticketDir, "SESSIONS.md")
	if common.FileExists(sessionsFile) {
		completionEntry := fmt.Sprintf("\n\n---\n\n## Ticket Completed: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))
		completionEntry += fmt.Sprintf("**Status:** Completed\n\n")
		if branch != "" {
			completionEntry += fmt.Sprintf("**Branch:** %s\n\n", branch)
		}
		if len(ticket.Commits) > 0 {
			completionEntry += "**Commits:**\n"
			for _, commit := range ticket.Commits {
				completionEntry += fmt.Sprintf("- %s\n", commit)
			}
			completionEntry += "\n"
		}
		if len(ticket.PullRequests) > 0 {
			completionEntry += "**Pull Requests:**\n"
			for _, pr := range ticket.PullRequests {
				completionEntry += fmt.Sprintf("- %s\n", pr)
			}
			completionEntry += "\n"
		}

		// Append to SESSIONS.md
		f, err := os.OpenFile(sessionsFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err == nil {
			defer f.Close()
			if _, err := f.WriteString(completionEntry); err != nil {
				warningMsg(fmt.Sprintf("Failed to append to SESSIONS.md: %v", err))
			} else {
				successMsg("Updated SESSIONS.md with completion info")
			}
		} else {
			warningMsg(fmt.Sprintf("Failed to open SESSIONS.md: %v", err))
		}
	}

	// Move ticket to archived with unique name (handle existing archives)
	baseArchivedDir := filepath.Join(cfgMgr.GetContextsPath(), "_archived", fmt.Sprintf("%s_%s", time.Now().Format("2006-01-02"), ticketID))
	archivedDir := baseArchivedDir
	suffix := 1
	for common.FileExists(archivedDir) {
		archivedDir = fmt.Sprintf("%s-%d", baseArchivedDir, suffix)
		suffix++
	}

	// Copy concrete files from primary project to archive
	if ticket.PrimaryContextName != "" {
		// V2 Architecture: copy files, so create directory first
		if err := common.EnsureDir(archivedDir); err != nil {
			return fmt.Errorf("failed to create archived directory: %w", err)
		}
		primaryProject := cfg.GetProject(ticket.PrimaryContextName)
		if primaryProject != nil {
			primaryTicketFile := filepath.Join(primaryProject.ProjectPath, ticketID+".md")
			primarySessionsFile := filepath.Join(primaryProject.ProjectPath, "SESSIONS.md")

			// Copy concrete files to archive
			if common.FileExists(primaryTicketFile) {
				if err := common.CopyFile(primaryTicketFile, filepath.Join(archivedDir, ticketID+".md")); err != nil {
					warningMsg(fmt.Sprintf("Failed to copy ticket file to archive: %v", err))
				} else {
					successMsg("Copied ticket file to archive")
					// Remove from primary project
					os.Remove(primaryTicketFile)
				}
			}

			if common.FileExists(primarySessionsFile) {
				if err := common.CopyFile(primarySessionsFile, filepath.Join(archivedDir, "SESSIONS.md")); err != nil {
					warningMsg(fmt.Sprintf("Failed to copy sessions file to archive: %v", err))
				} else {
					// Remove from primary project
					os.Remove(primarySessionsFile)
				}
			}

			// Remove symlinks from all secondary projects
			for _, lp := range ticket.LinkedProjects {
				if lp.ContextName != ticket.PrimaryContextName {
					secondaryTicketFile := filepath.Join(lp.ProjectPath, ticketID+".md")
					secondarySessionsFile := filepath.Join(lp.ProjectPath, "SESSIONS.md")
					common.RemoveSymlink(secondaryTicketFile)
					common.RemoveSymlink(secondarySessionsFile)
				}
			}

			// Remove symlinks from data dir
			dataTicketFile := filepath.Join(ticketDir, ticketID+".md")
			dataSessionsFile := filepath.Join(ticketDir, "SESSIONS.md")
			common.RemoveSymlink(dataTicketFile)
			common.RemoveSymlink(dataSessionsFile)

			// Remove ticket directory (should be empty now)
			os.RemoveAll(ticketDir)
			successMsg("Moved ticket to archived")
		}
	} else {
		// V1 Architecture: move entire directory
		// Ensure parent _archived directory exists
		archivedParent := filepath.Join(cfgMgr.GetContextsPath(), "_archived")
		if err := common.EnsureDir(archivedParent); err != nil {
			return fmt.Errorf("failed to create archived parent directory: %w", err)
		}

		if err := os.Rename(ticketDir, archivedDir); err != nil {
			return fmt.Errorf("failed to move ticket to archived: %w", err)
		}
		successMsg("Moved ticket to archived")
	}

	// Update config: move from active to archived
	// NOTE: We only save config once at the end to ensure atomicity.
	// If archiving fails, we don't want to persist a partially completed state.
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

	// Remove from working tickets if present
	if cfg.IsCurrentlyWorking(ticketID) {
		cfg.RemoveCurrentWorkingTicket(ticketID)
	}

	// Save config once at the end after all operations complete
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
	Use:   "abandon <ticket-id>",
	Short: "Mark ticket as abandoned",
	Long: `Mark a ticket as abandoned with optional reason.

Example:
  cctx ticket abandon TICKET-123
  cctx ticket abandon TICKET-123 --reason "Requirements changed"`,
	Args: cobra.ExactArgs(1),
	RunE: runTicketAbandon,
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

	// Git commit removed (no longer tracking in git)

	fmt.Println()
	successMsg(fmt.Sprintf("Ticket %s marked as abandoned", ticketID))
	infoMsg("You can archive it with: cctx ticket archive " + ticketID)

	// Remove from working tickets if present
	if cfg.IsCurrentlyWorking(ticketID) {
		cfg.RemoveCurrentWorkingTicket(ticketID)
		if err := cfgMgr.Save(cfg); err != nil {
			warningMsg(fmt.Sprintf("Failed to remove from working tickets: %v", err))
		}
	}

	return nil
}

// ticket archive subcommand
var ticketArchiveCmd = &cobra.Command{
	Use:   "archive <ticket-id>",
	Short: "Archive completed/abandoned ticket",
	Long: `Archive a completed or abandoned ticket.

Moves ticket to archived directory, removes symlinks from all projects,
and generates documentation for completed tickets.

Example:
  cctx ticket archive TICKET-123`,
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

	// Move ticket to archived with unique name (handle existing archives)
	ticketDir := filepath.Join(cfgMgr.GetContextsPath(), "_tickets", ticketID)
	baseArchivedDir := filepath.Join(cfgMgr.GetContextsPath(), "_archived", fmt.Sprintf("%s_%s", time.Now().Format("2006-01-02"), ticketID))
	archivedDir := baseArchivedDir
	suffix := 1
	for common.FileExists(archivedDir) {
		archivedDir = fmt.Sprintf("%s-%d", baseArchivedDir, suffix)
		suffix++
	}

	if dryRun {
		dryRunMsg(fmt.Sprintf("Would move ticket to: %s", archivedDir))
	} else {
		if err := common.EnsureDir(archivedDir); err != nil {
			return fmt.Errorf("failed to create archived directory: %w", err)
		}

		// Copy concrete files from primary project to archive
		if ticket.PrimaryContextName != "" {
			primaryProject := cfg.GetProject(ticket.PrimaryContextName)
			if primaryProject != nil {
				primaryTicketFile := filepath.Join(primaryProject.ProjectPath, ticketID+".md")
				primarySessionsFile := filepath.Join(primaryProject.ProjectPath, "SESSIONS.md")

				// Copy concrete files to archive
				if common.FileExists(primaryTicketFile) {
					if err := common.CopyFile(primaryTicketFile, filepath.Join(archivedDir, ticketID+".md")); err != nil{
						warningMsg(fmt.Sprintf("Failed to copy ticket file to archive: %v", err))
					} else {
						successMsg("Copied ticket file to archive")
						// Remove from primary project
						os.Remove(primaryTicketFile)
					}
				}

				if common.FileExists(primarySessionsFile) {
					if err := common.CopyFile(primarySessionsFile, filepath.Join(archivedDir, "SESSIONS.md")); err != nil {
						warningMsg(fmt.Sprintf("Failed to copy sessions file to archive: %v", err))
					} else {
						// Remove from primary project
						os.Remove(primarySessionsFile)
					}
				}

				// Remove symlinks from data dir
				dataTicketFile := filepath.Join(ticketDir, ticketID+".md")
				dataSessionsFile := filepath.Join(ticketDir, "SESSIONS.md")
				common.RemoveSymlink(dataTicketFile)
				common.RemoveSymlink(dataSessionsFile)

				// Remove ticket directory (should be empty now)
				os.RemoveAll(ticketDir)
			}
		} else {
			// V1 Architecture: Move entire directory
			if err := os.Rename(ticketDir, archivedDir); err != nil {
				return fmt.Errorf("failed to move ticket to archived: %w", err)
			}
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
- .clauderc entries in projects

This action cannot be undone.

Example:
  cctx ticket delete TICKET-123
  cctx ticket delete TICKET-123 --force  # Skip confirmation`,
	Args: cobra.ExactArgs(1),
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

	// Determine ticket directory
	var ticketDir string
	if ticket != nil {
		if ticket.ArchivedPath != "" {
			ticketDir = filepath.Join(dataDir, ticket.ArchivedPath)
		} else {
			ticketDir = filepath.Join(cfgMgr.GetContextsPath(), "_tickets", ticketID)
		}
	} else {
		ticketDir = filepath.Join(cfgMgr.GetContextsPath(), "_tickets", ticketID)
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

	// Remove symlinks from ALL managed projects (not just LinkedProjects)
	// This handles cases where symlinks exist but weren't tracked in LinkedProjects
	for _, project := range cfg.ManagedProjects {
		symlinkPath := filepath.Join(project.ProjectPath, ticketID+".md")
		sessionsSymlinkPath := filepath.Join(project.ProjectPath, "SESSIONS.md")

		// Check if ticket symlink exists
		ticketSymlinkExists := common.FileExists(symlinkPath)
		sessionsSymlinkExists := common.FileExists(sessionsSymlinkPath)

		if !ticketSymlinkExists && !sessionsSymlinkExists {
			continue
		}

		if dryRun {
			if ticketSymlinkExists {
				dryRunMsg(fmt.Sprintf("Would remove ticket file from %s", project.ContextName))
				dryRunMsg("Would remove from .clauderc")
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

				// Update .clauderc
				rcMgr := clauderc.NewManager(project.ProjectPath)
				if err := rcMgr.RemoveFile(ticketID+".md", dryRun); err != nil {
					warningMsg(fmt.Sprintf("Failed to update .clauderc in %s: %v", project.ContextName, err))
				}
			}

			// Remove SESSIONS.md file (symlink or concrete)
			if sessionsSymlinkExists {
				var err error
				if common.IsSymlink(sessionsSymlinkPath) {
					err = common.RemoveSymlink(sessionsSymlinkPath)
				} else {
					err = os.Remove(sessionsSymlinkPath)
				}
				if err != nil {
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

				// Update .clauderc in current directory
				rcMgr := clauderc.NewManager(cwd)
				if err := rcMgr.RemoveFile(ticketID+".md", dryRun); err != nil {
					warningMsg(fmt.Sprintf("Failed to update .clauderc: %v", err))
				}
			}
		}

		// Also check for SESSIONS.md file
		if common.FileExists(sessionsSymlinkPath) {
			if dryRun {
				dryRunMsg(fmt.Sprintf("Would remove SESSIONS.md file from current directory"))
			} else {
				var err error
				if common.IsSymlink(sessionsSymlinkPath) {
					err = common.RemoveSymlink(sessionsSymlinkPath)
				} else {
					err = os.Remove(sessionsSymlinkPath)
				}
				if err != nil {
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

		// Move ticket directory to archived with unique name (handle existing archives)
		ticketDir := filepath.Join(cfgMgr.GetContextsPath(), "_tickets", ticketID)
		baseArchivedDir := filepath.Join(cfgMgr.GetContextsPath(), "_archived",
			fmt.Sprintf("%s_%s", time.Now().Format("2006-01-02"), ticketID))
		archivedDir := baseArchivedDir
		suffix := 1
		for common.FileExists(archivedDir) {
			archivedDir = fmt.Sprintf("%s-%d", baseArchivedDir, suffix)
			suffix++
		}

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