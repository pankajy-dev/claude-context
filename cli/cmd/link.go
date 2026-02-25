package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pankaj/claude-context/internal/clauderc"
	"github.com/pankaj/claude-context/internal/common"
	"github.com/pankaj/claude-context/internal/config"
	"github.com/spf13/cobra"
)

var linkCmd = &cobra.Command{
	Use:   "link <project-path> [context-name]",
	Short: "Link a project to the context manager",
	Long: `Link a project directory to the Claude Context Manager.

This creates a centralized context file and symlinks it to your project directory.
If the project already has a claude.md file, you'll be prompted to import it or
create a backup.

The .clauderc file will be automatically created/updated to include claude.md
and any global context files.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runLink,
}

func init() {
	rootCmd.AddCommand(linkCmd)
}

func runLink(cmd *cobra.Command, args []string) error {
	projectPath := args[0]
	contextName := ""
	if len(args) > 1 {
		contextName = args[1]
	}

	// Normalize project path
	projectPath, err := common.NormalizePath(projectPath)
	if err != nil {
		return fmt.Errorf("invalid project path: %w", err)
	}

	// Check if project directory exists
	if !common.DirExists(projectPath) {
		return fmt.Errorf("project directory does not exist: %s", projectPath)
	}

	// Default context name to project directory name
	if contextName == "" {
		contextName = filepath.Base(projectPath)
	}

	// Get data directory
	dataDir := GetDataDirOrExit()

	// Load config
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if context name already exists
	if cfg.GetProject(contextName) != nil {
		return fmt.Errorf("context name already exists: %s", contextName)
	}

	// Create context directory
	contextDir := filepath.Join(cfgMgr.GetContextsPath(), contextName)
	contextFile := filepath.Join(contextDir, "claude.md")

	if dryRun {
		dryRunMsg(fmt.Sprintf("Would create context directory: %s", contextDir))
		dryRunMsg(fmt.Sprintf("Would create context file: %s", contextFile))
	} else {
		if err := common.EnsureDir(contextDir); err != nil {
			return fmt.Errorf("failed to create context directory: %w", err)
		}
		successMsg(fmt.Sprintf("Created context directory: %s", contextDir))
	}

	// Handle existing claude.md in project
	projectClaudeMD := filepath.Join(projectPath, "claude.md")

	// Concrete file stays in project, data dir symlinks to it
	if common.FileExists(projectClaudeMD) && !common.IsSymlink(projectClaudeMD) {
		infoMsg("Project already has claude.md file - using existing")
		if !dryRun {
			successMsg("Using existing claude.md")
		}
	} else if common.IsSymlink(projectClaudeMD) {
		// Old symlink exists - remove it
		infoMsg("Removing old symlink")
		if !dryRun {
			if err := common.RemoveSymlink(projectClaudeMD); err != nil {
				return fmt.Errorf("failed to remove old symlink: %w", err)
			}
		}
	}

	// Create concrete file in project if it doesn't exist
	if !common.FileExists(projectClaudeMD) {
		if !dryRun {
			template := fmt.Sprintf("# %s\n\n", contextName)
			if err := os.WriteFile(projectClaudeMD, []byte(template), 0644); err != nil {
				return fmt.Errorf("failed to create project claude.md: %w", err)
			}
			successMsg("Created claude.md in project")
		}
	}

	// Create symlink in data dir pointing to project
	if dryRun {
		dryRunMsg(fmt.Sprintf("Would create data dir symlink: %s -> %s", contextFile, projectClaudeMD))
	} else {
		if err := common.CreateSymlink(projectClaudeMD, contextFile); err != nil {
			return fmt.Errorf("failed to create data dir symlink: %w", err)
		}
		successMsg("Created data directory symlink")
	}

	// Link global contexts if enabled
	var linkedGlobals []string
	for _, gc := range cfg.GlobalContexts {
		if gc.Enabled {
			globalFile := filepath.Join(dataDir, gc.Path)
			projectGlobal := filepath.Join(projectPath, filepath.Base(gc.Path))

			if dryRun {
				dryRunMsg(fmt.Sprintf("Would create global symlink: %s", filepath.Base(gc.Path)))
			} else {
				if err := common.CreateSymlink(globalFile, projectGlobal); err != nil {
					warningMsg(fmt.Sprintf("Failed to create global symlink: %v", err))
				} else {
					successMsg(fmt.Sprintf("Created global symlink: %s", filepath.Base(gc.Path)))
					linkedGlobals = append(linkedGlobals, gc.Name)
				}
			}
		}
	}

	// Update .clauderc to include symlinked files
	// Only create/update .clauderc if it already exists OR we have global contexts to add
	if !dryRun {
		rcMgr := clauderc.NewManager(projectPath)

		// Check if we have any enabled global contexts
		hasEnabledGlobals := false
		for _, gc := range cfg.GlobalContexts {
			if gc.Enabled {
				hasEnabledGlobals = true
				break
			}
		}

		// Only update .clauderc if it exists OR we have globals
		// If only claude.md and no .clauderc exists, don't create it (auto-loaded)
		if rcMgr.Exists() || hasEnabledGlobals {
			// Add claude.md to .clauderc
			if err := rcMgr.AddFile("claude.md", false); err != nil {
				warningMsg(fmt.Sprintf("Failed to update .clauderc: %v", err))
			} else {
				successMsg("Updated .clauderc")
			}

			// Add global contexts to .clauderc
			for _, gc := range cfg.GlobalContexts {
				if gc.Enabled {
					globalFileName := filepath.Base(gc.Path)
					if err := rcMgr.AddFile(globalFileName, false); err != nil {
						warningMsg(fmt.Sprintf("Failed to add %s to .clauderc: %v", globalFileName, err))
					}
				}
			}
		}
	}

	// Add project to config
	project := config.Project{
		ContextName:   contextName,
		ProjectPath:   projectPath,
		ContextPath:   filepath.Join("contexts", contextName, "claude.md"),
		CreatedAt:     time.Now(),
		LastModified:  time.Now(),
		Status:        "active",
		LinkedGlobals: linkedGlobals,
	}

	if !dryRun {
		cfg.AddProject(project)
		if err := cfgMgr.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		successMsg("Updated configuration")
	}

	// Git commit removed (no longer tracking in git)

	if !dryRun {
		fmt.Println()
		successMsg(fmt.Sprintf("Successfully linked project: %s", contextName))
		infoMsg(fmt.Sprintf("Project path: %s", projectPath))
		infoMsg(fmt.Sprintf("Context file: %s", contextFile))
	}

	return nil
}

