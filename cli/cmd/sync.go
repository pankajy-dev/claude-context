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

var syncCmd = &cobra.Command{
	Use:   "sync <project-path> [context-name]",
	Short: "Import existing claude.md from a project",
	Long: `Import an existing claude.md file from a project.

The file is copied to the central repository and replaced with a symlink.
Requires confirmation before proceeding.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runSync,
}

func init() {
	rootCmd.AddCommand(syncCmd)
}

func runSync(cmd *cobra.Command, args []string) error {
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

	// Check if claude.md exists in project
	projectClaudeMD := filepath.Join(projectPath, "claude.md")
	if !common.FileExists(projectClaudeMD) {
		return fmt.Errorf("claude.md not found in project: %s", projectPath)
	}

	if common.IsSymlink(projectClaudeMD) {
		return fmt.Errorf("claude.md is already a symlink")
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

	infoMsg(fmt.Sprintf("Importing claude.md from: %s", projectPath))
	infoMsg(fmt.Sprintf("Context name: %s", contextName))
	fmt.Println()

	// Confirmation required
	if !dryRun {
		if !common.Confirm("Proceed with sync?", false) {
			infoMsg("Operation cancelled")
			return nil
		}
		fmt.Println()
	}

	// Create context directory
	contextDir := filepath.Join(cfgMgr.GetContextsPath(), contextName)
	contextFile := filepath.Join(contextDir, "claude.md")

	if dryRun {
		dryRunMsg(fmt.Sprintf("Would create context directory: %s", contextDir))
		dryRunMsg(fmt.Sprintf("Would copy claude.md to: %s", contextFile))
		dryRunMsg(fmt.Sprintf("Would backup original file"))
		dryRunMsg(fmt.Sprintf("Would replace with symlink"))
	} else {
		// Create context directory
		if err := common.EnsureDir(contextDir); err != nil {
			return fmt.Errorf("failed to create context directory: %w", err)
		}
		successMsg(fmt.Sprintf("Created context directory"))

		// Copy existing claude.md
		content, err := os.ReadFile(projectClaudeMD)
		if err != nil {
			return fmt.Errorf("failed to read existing claude.md: %w", err)
		}

		if err := os.WriteFile(contextFile, content, 0644); err != nil {
			return fmt.Errorf("failed to write context file: %w", err)
		}
		successMsg("Imported claude.md content")

		// Backup original file
		backupFile := projectClaudeMD + ".backup"
		if err := os.Rename(projectClaudeMD, backupFile); err != nil {
			return fmt.Errorf("failed to backup original file: %w", err)
		}
		successMsg(fmt.Sprintf("Backed up original: %s", backupFile))

		// Create symlink
		if err := common.CreateSymlink(contextFile, projectClaudeMD); err != nil {
			// Restore backup on failure
			os.Rename(backupFile, projectClaudeMD)
			return fmt.Errorf("failed to create symlink: %w", err)
		}
		successMsg("Created symlink")

		// Link global contexts if enabled
		for _, gc := range cfg.GlobalContexts {
			if gc.Enabled {
				globalFile := filepath.Join(dataDir, gc.Path)
				projectGlobal := filepath.Join(projectPath, filepath.Base(gc.Path))

				if err := common.CreateSymlink(globalFile, projectGlobal); err != nil {
					warningMsg(fmt.Sprintf("Failed to create global symlink: %v", err))
				} else {
					successMsg(fmt.Sprintf("Created global symlink: %s", filepath.Base(gc.Path)))
				}
			}
		}

		// Update .clauderc to include claude.md
		// Only create/update .clauderc if it already exists OR we have global contexts to add
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

		// Add project to config
		project := config.Project{
			ContextName:  contextName,
			ProjectPath:  projectPath,
			ContextPath:  filepath.Join("contexts", contextName, "claude.md"),
			CreatedAt:    time.Now(),
			LastModified: time.Now(),
			Status:       "active",
		}

		cfg.AddProject(project)
		if err := cfgMgr.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		successMsg("Updated configuration")
	}

	// Git commit removed (no longer tracking in git)

	if !dryRun {
		fmt.Println()
		successMsg(fmt.Sprintf("Successfully synced project: %s", contextName))
		infoMsg(fmt.Sprintf("Context file: %s", contextFile))
		infoMsg(fmt.Sprintf("Original backed up to: %s.backup", projectClaudeMD))
	}

	return nil
}
