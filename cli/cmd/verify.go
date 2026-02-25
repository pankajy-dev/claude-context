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
	autoFix bool
)

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify health of all managed projects",
	Long: `Verify the health of all managed projects.

Checks:
- Symlinks are valid and point to correct locations (claude.md, tickets, globals)
- Config entries have corresponding context files
- No orphaned context files (not in config)
- Project directories still exist
- Global context symlinks (if enabled)
- Ticket symlinks for active tickets

With --fix flag, prompts you to either:
  1. Recreate missing symlinks (if accidentally deleted)
  2. Remove from config (if intentionally unlinked)`,
	RunE: runVerify,
}

func init() {
	rootCmd.AddCommand(verifyCmd)
	verifyCmd.Flags().BoolVar(&autoFix, "fix", false, "Automatically fix issues where possible (requires confirmation)")
}

type verifyIssue struct {
	project     string
	issueType   string
	description string
	fixable     bool
}

func runVerify(cmd *cobra.Command, args []string) error {
	// Get data directory
	dataDir := GetDataDirOrExit()

	// Load config
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	infoMsg("Verifying all managed projects...")
	fmt.Println()

	issues := []verifyIssue{}
	healthyCount := 0

	// Check each project
	for _, project := range cfg.ManagedProjects {
		fmt.Printf("Checking: %s\n", project.ContextName)

		// Track issues before checking this project
		issuesBeforeCheck := len(issues)

		// Check if project directory exists
		if !common.DirExists(project.ProjectPath) {
			issues = append(issues, verifyIssue{
				project:     project.ContextName,
				issueType:   "missing-dir",
				description: fmt.Sprintf("Project directory does not exist: %s", project.ProjectPath),
				fixable:     false,
			})
			fmt.Printf("  ✗ Project directory missing\n")
			continue
		}

		// Check claude.md: project has concrete file, data dir has symlink
		claudeMD := filepath.Join(project.ProjectPath, "claude.md")
		contextFile := filepath.Join(dataDir, project.ContextPath)

		if !common.FileExists(claudeMD) {
			issues = append(issues, verifyIssue{
				project:     project.ContextName,
				issueType:   "missing-file",
				description: "claude.md file is missing (should be concrete file)",
				fixable:     true,
			})
			fmt.Printf("  ✗ claude.md missing\n")
		} else if common.IsSymlink(claudeMD) {
			issues = append(issues, verifyIssue{
				project:     project.ContextName,
				issueType:   "unexpected-symlink",
				description: "claude.md is a symlink (expected concrete file)",
				fixable:     false,
			})
			fmt.Printf("  ✗ claude.md should be concrete file, not symlink\n")
		} else {
			// Concrete file exists - check data dir symlink
			if !common.FileExists(contextFile) {
				issues = append(issues, verifyIssue{
					project:     project.ContextName,
					issueType:   "missing-data-symlink",
					description: "Data dir symlink is missing",
					fixable:     true,
				})
				fmt.Printf("  ✗ Data dir symlink missing\n")
			} else if !common.IsSymlink(contextFile) {
				issues = append(issues, verifyIssue{
					project:     project.ContextName,
					issueType:   "data-not-symlink",
					description: "Data dir file is not a symlink",
					fixable:     false,
				})
				fmt.Printf("  ✗ Data dir file should be symlink\n")
			} else {
				target, _ := common.SymlinkTarget(contextFile)
				if target != claudeMD {
					issues = append(issues, verifyIssue{
						project:     project.ContextName,
						issueType:   "wrong-target",
						description: fmt.Sprintf("Data dir symlink points to wrong target: %s", target),
						fixable:     true,
					})
					fmt.Printf("  ✗ Data dir symlink wrong target\n")
				} else {
					fmt.Printf("  ✓ claude.md OK\n")
				}
			}
		}

		// Check global context symlinks (based on tracked links in project)
		for _, globalName := range project.LinkedGlobals {
			gc := cfg.GetGlobalContext(globalName)
			if gc == nil {
				issues = append(issues, verifyIssue{
					project:     project.ContextName,
					issueType:   "unknown-global",
					description: fmt.Sprintf("tracked global '%s' not found in config", globalName),
					fixable:     false,
				})
				fmt.Printf("  ✗ %s: not found in config\n", globalName)
				continue
			}

			globalFile := filepath.Join(project.ProjectPath, filepath.Base(gc.Path))
			globalTarget := filepath.Join(dataDir, gc.Path)

			if !common.FileExists(globalFile) {
				issues = append(issues, verifyIssue{
					project:     project.ContextName,
					issueType:   "missing-global",
					description: fmt.Sprintf("%s symlink is missing", filepath.Base(gc.Path)),
					fixable:     true,
				})
				fmt.Printf("  ✗ %s symlink missing\n", filepath.Base(gc.Path))
			} else if !common.IsSymlink(globalFile) {
				issues = append(issues, verifyIssue{
					project:     project.ContextName,
					issueType:   "global-not-symlink",
					description: fmt.Sprintf("%s exists but is not a symlink", filepath.Base(gc.Path)),
					fixable:     false,
				})
				fmt.Printf("  ✗ %s is not a symlink\n", filepath.Base(gc.Path))
			} else {
				target, _ := common.SymlinkTarget(globalFile)
				if target != globalTarget {
					issues = append(issues, verifyIssue{
						project:     project.ContextName,
						issueType:   "global-wrong-target",
						description: fmt.Sprintf("%s points to wrong target", filepath.Base(gc.Path)),
						fixable:     true,
					})
					fmt.Printf("  ✗ %s wrong target\n", filepath.Base(gc.Path))
				} else {
					fmt.Printf("  ✓ %s OK\n", filepath.Base(gc.Path))
				}
			}
		}

		// Check ticket symlinks (based on active tickets linked to this project)
		for _, ticket := range cfg.Tickets.Active {
			isLinked := false
			for _, lp := range ticket.LinkedProjects {
				if lp.ContextName == project.ContextName {
					isLinked = true
					break
				}
			}

			if !isLinked {
				continue
			}

			// Check if ticket symlink exists
			ticketFile := filepath.Join(project.ProjectPath, ticket.TicketID+".md")
			ticketTarget := filepath.Join(dataDir, "contexts", "_tickets", ticket.TicketID, "ticket.md")

			if !common.FileExists(ticketFile) {
				issues = append(issues, verifyIssue{
					project:     project.ContextName,
					issueType:   "missing-ticket",
					description: fmt.Sprintf("ticket %s symlink is missing (config says it's linked)", ticket.TicketID),
					fixable:     true,
				})
				fmt.Printf("  ✗ %s.md symlink missing (linked in config)\n", ticket.TicketID)
			} else if !common.IsSymlink(ticketFile) {
				issues = append(issues, verifyIssue{
					project:     project.ContextName,
					issueType:   "ticket-not-symlink",
					description: fmt.Sprintf("%s.md exists but is not a symlink", ticket.TicketID),
					fixable:     false,
				})
				fmt.Printf("  ✗ %s.md is not a symlink\n", ticket.TicketID)
			} else {
				target, _ := common.SymlinkTarget(ticketFile)
				if target != ticketTarget {
					issues = append(issues, verifyIssue{
						project:     project.ContextName,
						issueType:   "ticket-wrong-target",
						description: fmt.Sprintf("%s.md points to wrong target", ticket.TicketID),
						fixable:     true,
					})
					fmt.Printf("  ✗ %s.md wrong target\n", ticket.TicketID)
				} else {
					fmt.Printf("  ✓ %s.md OK\n", ticket.TicketID)
				}
			}
		}

		// If no new issues were added for this project, it's healthy
		if len(issues) == issuesBeforeCheck {
			healthyCount++
		}
		fmt.Println()
	}

	// Check for orphaned context directories
	contextsDir := cfgMgr.GetContextsPath()
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
				issues = append(issues, verifyIssue{
					project:     entry.Name(),
					issueType:   "orphaned",
					description: "Context directory exists but not in config",
					fixable:     true,
				})
			}
		}
	}

	// Summary
	fmt.Println("=" + string(make([]byte, 50)))
	fmt.Printf("Verification Summary\n")
	fmt.Println("=" + string(make([]byte, 50)))
	fmt.Printf("Total projects: %d\n", len(cfg.ManagedProjects))
	fmt.Printf("Healthy: %d\n", healthyCount)
	fmt.Printf("Issues found: %d\n", len(issues))
	fmt.Println()

	if len(issues) == 0 {
		successMsg("All projects are healthy!")
		return nil
	}

	// Show issues
	fmt.Println("Issues:")
	for i, issue := range issues {
		fmt.Printf("%d. [%s] %s: %s\n", i+1, issue.project, issue.issueType, issue.description)
		if issue.fixable {
			fmt.Printf("   (fixable)\n")
		}
	}
	fmt.Println()

	// Auto-fix if requested
	if autoFix {
		fixableCount := 0
		for _, issue := range issues {
			if issue.fixable {
				fixableCount++
			}
		}

		if fixableCount == 0 {
			infoMsg("No fixable issues found")
			return nil
		}

		infoMsg(fmt.Sprintf("Found %d fixable issues", fixableCount))

		// Confirmation required (unless dry-run)
		if !dryRun {
			if !common.Confirm("Proceed with auto-fix?", false) {
				infoMsg("Auto-fix cancelled")
				return nil
			}
			fmt.Println()
		}

		// Fix issues
		fixed := 0
		configChanged := false
		for _, issue := range issues {
			if !issue.fixable {
				continue
			}

			project := cfg.GetProject(issue.project)
			if project == nil && issue.issueType != "orphaned" {
				continue
			}

			switch issue.issueType {
			case "missing-symlink", "wrong-target", "broken-symlink":
				// Recreate claude.md symlink
				claudeMD := filepath.Join(project.ProjectPath, "claude.md")
				contextFile := filepath.Join(dataDir, project.ContextPath)

				// Ensure context file exists
				contextDir := filepath.Dir(contextFile)
				if !dryRun {
					if err := common.EnsureDir(contextDir); err != nil {
						warningMsg(fmt.Sprintf("Failed to create context dir for %s: %v", issue.project, err))
						continue
					}
					if !common.FileExists(contextFile) {
						// Create empty context file
						if err := os.WriteFile(contextFile, []byte(fmt.Sprintf("# %s\n\n", project.ContextName)), 0644); err != nil {
							warningMsg(fmt.Sprintf("Failed to create context file for %s: %v", issue.project, err))
							continue
						}
					}
					if err := common.CreateSymlink(contextFile, claudeMD); err != nil {
						warningMsg(fmt.Sprintf("Failed to fix symlink for %s: %v", issue.project, err))
						continue
					}
					successMsg(fmt.Sprintf("Fixed claude.md symlink for %s", issue.project))
					fixed++
				} else {
					dryRunMsg(fmt.Sprintf("Would fix claude.md symlink for %s", issue.project))
				}

			case "missing-context":
				// Create empty context file
				contextFile := filepath.Join(dataDir, project.ContextPath)
				contextDir := filepath.Dir(contextFile)
				if !dryRun {
					if err := common.EnsureDir(contextDir); err != nil {
						warningMsg(fmt.Sprintf("Failed to create context dir for %s: %v", issue.project, err))
						continue
					}
					if err := os.WriteFile(contextFile, []byte(fmt.Sprintf("# %s\n\n", project.ContextName)), 0644); err != nil {
						warningMsg(fmt.Sprintf("Failed to create context file for %s: %v", issue.project, err))
						continue
					}
					successMsg(fmt.Sprintf("Created context file for %s", issue.project))
					fixed++
				} else {
					dryRunMsg(fmt.Sprintf("Would create context file for %s", issue.project))
				}

			case "missing-global", "global-wrong-target":
				// Find which global is missing
				for _, globalName := range project.LinkedGlobals {
					gc := cfg.GetGlobalContext(globalName)
					if gc == nil {
						continue
					}

					globalFile := filepath.Join(project.ProjectPath, filepath.Base(gc.Path))
					globalTarget := filepath.Join(dataDir, gc.Path)

					// Check if this is the symlink that needs fixing
					needsFix := false
					if !common.FileExists(globalFile) || !common.IsSymlink(globalFile) {
						needsFix = true
					} else {
						target, _ := common.SymlinkTarget(globalFile)
						if target != globalTarget {
							needsFix = true
						}
					}

					if needsFix {
						if !dryRun {
							fmt.Println()
							warningMsg(fmt.Sprintf("Global context %s symlink missing in %s", globalName, issue.project))
							fmt.Println("Options:")
							fmt.Println("  1. Recreate symlink (you accidentally deleted it)")
							fmt.Println("  2. Remove from config (you intentionally unlinked it)")
							fmt.Print("\nEnter choice [1-2]: ")

							var choice string
							fmt.Scanln(&choice)

							switch choice {
							case "1":
								// Recreate symlink
								if common.FileExists(globalFile) {
									os.Remove(globalFile)
								}

								if err := common.CreateSymlink(globalTarget, globalFile); err != nil {
									warningMsg(fmt.Sprintf("Failed to recreate symlink: %v", err))
									continue
								}
								successMsg(fmt.Sprintf("Recreated %s symlink", filepath.Base(gc.Path)))
								fixed++

							case "2":
								// Remove from project's linked globals
								newLinkedGlobals := []string{}
								for _, g := range project.LinkedGlobals {
									if g != globalName {
										newLinkedGlobals = append(newLinkedGlobals, g)
									}
								}
								project.LinkedGlobals = newLinkedGlobals
								configChanged = true
								successMsg(fmt.Sprintf("Unlinked %s from %s", globalName, issue.project))
								fixed++

							default:
								infoMsg(fmt.Sprintf("Skipped fixing %s", globalName))
							}
						} else {
							dryRunMsg(fmt.Sprintf("Would prompt for action on missing global %s", globalName))
						}
					}
				}

			case "missing-ticket", "ticket-wrong-target":
				// Extract ticket ID from issue description
				ticketID := ""
				for _, ticket := range cfg.Tickets.Active {
					for _, lp := range ticket.LinkedProjects {
						if lp.ContextName == issue.project {
							ticketID = ticket.TicketID
							break
						}
					}
					if ticketID != "" {
						break
					}
				}

				if ticketID == "" {
					continue
				}

				// Ask user what to do
				if !dryRun {
					fmt.Println()
					warningMsg(fmt.Sprintf("Ticket %s symlink missing in %s", ticketID, issue.project))
					fmt.Println("Options:")
					fmt.Println("  1. Recreate symlink (you accidentally deleted it)")
					fmt.Println("  2. Remove from config (you intentionally unlinked it)")
					fmt.Print("\nEnter choice [1-2]: ")

					var choice string
					fmt.Scanln(&choice)

					switch choice {
					case "1":
						// Recreate symlink
						ticketFile := filepath.Join(project.ProjectPath, ticketID+".md")
						ticketTarget := filepath.Join(dataDir, "contexts", "_tickets", ticketID, "ticket.md")

						// Remove existing if wrong
						if common.FileExists(ticketFile) {
							os.Remove(ticketFile)
						}

						if err := common.CreateSymlink(ticketTarget, ticketFile); err != nil {
							warningMsg(fmt.Sprintf("Failed to recreate symlink: %v", err))
							continue
						}
						successMsg(fmt.Sprintf("Recreated %s.md symlink", ticketID))
						fixed++

					case "2":
						// Remove from config
						for i, ticket := range cfg.Tickets.Active {
							if ticket.TicketID == ticketID {
								// Remove project from ticket's linked projects
								newLinkedProjects := []config.LinkedProject{}
								for _, lp := range ticket.LinkedProjects {
									if lp.ContextName != issue.project {
										newLinkedProjects = append(newLinkedProjects, lp)
									}
								}
								cfg.Tickets.Active[i].LinkedProjects = newLinkedProjects
								configChanged = true
								successMsg(fmt.Sprintf("Removed %s from ticket %s config", issue.project, ticketID))
								fixed++
								break
							}
						}

					default:
						infoMsg(fmt.Sprintf("Skipped fixing %s", ticketID))
					}
				} else {
					dryRunMsg(fmt.Sprintf("Would prompt for action on missing ticket %s", ticketID))
				}

			case "orphaned":
				// Remove orphaned directory
				orphanedDir := filepath.Join(contextsDir, issue.project)
				if !dryRun {
					if err := os.RemoveAll(orphanedDir); err != nil {
						warningMsg(fmt.Sprintf("Failed to remove orphaned directory %s: %v", issue.project, err))
						continue
					}
					successMsg(fmt.Sprintf("Removed orphaned directory: %s", issue.project))
					fixed++
				} else {
					dryRunMsg(fmt.Sprintf("Would remove orphaned directory: %s", issue.project))
				}
			}
		}

		// Save config if changed
		if configChanged && !dryRun {
			if err := cfgMgr.Save(cfg); err != nil {
				warningMsg(fmt.Sprintf("Failed to save config: %v", err))
			} else {
				successMsg("Updated configuration")
			}
		}

		// Git commit removed (no longer tracking in git)

		fmt.Println()
		if !dryRun {
			successMsg(fmt.Sprintf("Fixed %d issues", fixed))
		}
	}

	return nil
}
