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

	// Confirmation required (unless dry-run)
	if !dryRun {
		if !common.Confirm("Proceed with unlink?", false) {
			infoMsg("Operation cancelled")
			return nil
		}
		fmt.Println()
	}

	// Remove claude.md symlink
	claudeMD := filepath.Join(project.ProjectPath, "claude.md")
	if dryRun {
		dryRunMsg(fmt.Sprintf("Would remove symlink: %s", claudeMD))
	} else {
		if common.FileExists(claudeMD) {
			if err := common.RemoveSymlink(claudeMD); err != nil {
				warningMsg(fmt.Sprintf("Failed to remove claude.md symlink: %v", err))
			} else {
				successMsg("Removed claude.md symlink")
			}
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

	// Git commit
	commitMsg := fmt.Sprintf("Unlink project: %s", contextName)
	if err := common.GitCommit(dataDir, commitMsg, dryRun); err != nil {
		warningMsg(fmt.Sprintf("Failed to commit to git: %v", err))
	} else if !dryRun {
		successMsg("Committed changes to git")
	}

	if !dryRun {
		fmt.Println()
		successMsg(fmt.Sprintf("Successfully unlinked project: %s", contextName))
	}

	return nil
}
