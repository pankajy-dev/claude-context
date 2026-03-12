package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pankaj/claude-context/internal/common"
	"github.com/pankaj/claude-context/internal/config"
	"github.com/pankaj/claude-context/internal/templates"
	"github.com/spf13/cobra"
)

var (
	forceInit bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize cctx data directory (one-time setup)",
	Long: `Initialize the Claude Context Manager data directory.

⚠️  NOTE: This is NOT Claude Code's /init command!
This is a ONE-TIME setup for cctx itself.

By default, this creates ~/.cctx (or the directory specified by --data-dir flag
or CCTX_DATA_DIR environment variable).

You only need to run this ONCE after installing cctx.
If you've already run it, you don't need to run it again.

This command:
- Creates the directory structure (~/.cctx)
- Sets up initial config.json
- Copies template files
- Migrates from old installation if detected

After initialization, use:
  cctx link <project-path>  - Link your first project
  cctx list                 - List managed projects`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVarP(&forceInit, "force", "f", false, "Force initialization even if directory exists")
}

func runInit(cmd *cobra.Command, args []string) error {
	dataDir, err := GetDataDir()
	if err != nil {
		return err
	}

	// Check if already initialized
	configPath := filepath.Join(dataDir, "config.json")
	if common.FileExists(configPath) {
		if !forceInit {
			fmt.Println()
			successMsg(fmt.Sprintf("✓ Already initialized at: %s", dataDir))
			fmt.Println()
			infoMsg("This directory is ready to use. Common commands:")
			infoMsg("  cctx list          - List managed projects")
			infoMsg("  cctx link <path>   - Link a new project")
			infoMsg("  cctx verify        - Check symlink health")
			fmt.Println()
			warningMsg("NOTE: This is NOT the same as Claude Code's /init command")
			warningMsg("If you need to reinitialize (DANGEROUS), use: cctx init --force")
			return nil
		}

		// Force reinitialization - require explicit confirmation
		fmt.Println()
		warningMsg("⚠️  WARNING: Force reinitialization will:")
		warningMsg("   - Overwrite your existing config.json")
		warningMsg("   - Potentially lose all project links and ticket data")
		warningMsg("   - Require you to re-link all projects")
		fmt.Println()
		if !common.Confirm("Are you ABSOLUTELY SURE you want to reinitialize?", false) {
			infoMsg("Cancelled. Your existing setup is unchanged.")
			return nil
		}
		fmt.Println()
		warningMsg("Proceeding with reinitialization...")
	}

	// Check if directory exists but is not initialized
	if common.FileExists(dataDir) && !common.FileExists(configPath) {
		// Check if directory is empty or has cctx-related files only
		entries, err := os.ReadDir(dataDir)
		if err != nil {
			return fmt.Errorf("failed to read directory %s: %w", dataDir, err)
		}

		// Check for non-cctx files
		hasNonCctxFiles := false
		for _, entry := range entries {
			name := entry.Name()
			// Allow these names
			if name == "config.json" || name == "contexts" || name == "templates" {
				continue
			}
			hasNonCctxFiles = true
			break
		}

		if hasNonCctxFiles && !forceInit {
			return fmt.Errorf("directory %s exists and contains non-cctx files. Use --force to proceed or choose a different directory", dataDir)
		}
	}

	// Check for old installation that needs migration
	oldConfigPath := findOldConfig()
	if oldConfigPath != "" && oldConfigPath != configPath {
		infoMsg(fmt.Sprintf("Found old installation at %s", filepath.Dir(oldConfigPath)))
		return migrateOldInstallation(oldConfigPath, dataDir)
	}

	// Fresh initialization
	return initializeFresh(dataDir)
}

// findOldConfig searches for config.json in current directory and parents
// Returns empty string if not found or if it's not a cctx config
func findOldConfig() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}

	// Search up to 5 levels up (don't go too far)
	for i := 0; i < 5; i++ {
		configPath := filepath.Join(dir, "config.json")
		if common.FileExists(configPath) {
			// Check if it's a claude-context config
			data, err := os.ReadFile(configPath)
			if err == nil {
				var check map[string]interface{}
				if json.Unmarshal(data, &check) == nil {
					// Check for managed_projects field
					if _, ok := check["managed_projects"]; ok {
						return configPath
					}
				}
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return ""
}

// initializeFresh creates a new installation from scratch
func initializeFresh(dataDir string) error {
	infoMsg(fmt.Sprintf("Initializing %s...", dataDir))

	// Create directory structure
	dirs := []string{
		dataDir,
		filepath.Join(dataDir, "contexts"),
		filepath.Join(dataDir, "contexts", "_global"),
		filepath.Join(dataDir, "contexts", "_tickets"),
		filepath.Join(dataDir, "contexts", "_archived"),
		filepath.Join(dataDir, "templates"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Create initial config.json
	initialConfig := config.Config{
		ManagedProjects: []config.Project{},
		GlobalContexts:  []config.GlobalContext{},
		Tickets: config.TicketSection{
			Active:   []config.Ticket{},
			Archived: []config.Ticket{},
			Settings: config.TicketSettings{
				AutoArchive: false,
			},
		},
		Settings: config.Settings{
			AutoCommit:     false, // No git tracking
			BackupOnUnlink: true,
		},
	}

	configData, err := json.MarshalIndent(initialConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	configPath := filepath.Join(dataDir, "config.json")
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	// Copy all embedded templates to user directory (single source of truth)
	infoMsg("Copying default templates...")
	copied, err := templates.CopyAllEmbeddedTemplates(dataDir, false)
	if err != nil {
		warningMsg(fmt.Sprintf("Warning: failed to copy templates: %v", err))
		warningMsg("You can manually copy them later using: cctx global templates sync")
	} else {
		successMsg(fmt.Sprintf("Copied %d templates to %s/templates/", copied, dataDir))
	}

	successMsg(fmt.Sprintf("Initialized %s", dataDir))
	infoMsg("")
	infoMsg(fmt.Sprintf("Templates location: %s/templates/", dataDir))
	infoMsg("You can edit these templates directly - they are the source of truth")
	infoMsg("View templates: cctx global templates list")
	infoMsg("Reset a template: cctx global templates reset <name>")
	infoMsg("")
	infoMsg("Next steps:")
	infoMsg("  1. Link your first project: cctx link /path/to/project")
	infoMsg("  2. List managed projects: cctx list")
	infoMsg("  3. Create a ticket workspace: cctx ticket create TICKET-123")

	return nil
}

// migrateOldInstallation moves data from old structure to new data directory
func migrateOldInstallation(oldConfigPath, dataDir string) error {
	oldRoot := filepath.Dir(oldConfigPath)

	// Don't migrate if old and new are the same
	if oldRoot == dataDir {
		return initializeFresh(dataDir)
	}

	infoMsg(fmt.Sprintf("Migrating from %s to %s", oldRoot, dataDir))

	// Create data directory
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Copy config.json
	infoMsg("Copying config.json...")
	if err := copyFile(oldConfigPath, filepath.Join(dataDir, "config.json")); err != nil {
		return fmt.Errorf("failed to copy config: %w", err)
	}

	// Copy contexts directory
	oldContextsDir := filepath.Join(oldRoot, "contexts")
	if common.FileExists(oldContextsDir) {
		infoMsg("Copying contexts directory...")
		newContextsDir := filepath.Join(dataDir, "contexts")
		if err := copyDir(oldContextsDir, newContextsDir); err != nil {
			return fmt.Errorf("failed to copy contexts: %w", err)
		}
	}

	// Copy user-customized templates if they exist
	oldTemplatesDir := filepath.Join(oldRoot, "templates")
	newTemplatesDir := filepath.Join(dataDir, "templates")
	if common.FileExists(oldTemplatesDir) {
		infoMsg("Migrating user templates...")
		if err := copyDir(oldTemplatesDir, newTemplatesDir); err != nil {
			warningMsg(fmt.Sprintf("Failed to copy templates: %v", err))
			// Try to copy embedded templates as fallback
			if copied, err := templates.CopyAllEmbeddedTemplates(dataDir, false); err == nil {
				successMsg(fmt.Sprintf("Copied %d default templates as fallback", copied))
			}
		} else {
			successMsg("User templates migrated")
		}
	} else {
		// No old templates, copy embedded ones
		infoMsg("Copying default templates...")
		if copied, err := templates.CopyAllEmbeddedTemplates(dataDir, false); err == nil {
			successMsg(fmt.Sprintf("Copied %d default templates", copied))
		}
	}

	// Update all symlinks to point to new location
	infoMsg("Updating symlinks...")
	if err := updateSymlinks(dataDir); err != nil {
		warningMsg(fmt.Sprintf("Some symlinks may need manual updating: %v", err))
		warningMsg("Run 'cctx verify --fix' to fix broken symlinks")
	}

	successMsg("Migration complete!")
	infoMsg("")
	infoMsg(fmt.Sprintf("Old installation at %s can be safely removed", oldRoot))
	infoMsg("This repository now only contains code")

	return nil
}

// updateSymlinks recreates all symlinks to point to new data directory
func updateSymlinks(dataDir string) error {
	mgr := config.NewManager(dataDir)
	cfg, err := mgr.Load()
	if err != nil {
		return err
	}

	errors := []string{}

	// Update project symlinks
	for _, project := range cfg.ManagedProjects {
		// Update context symlink - extract filename from ContextPath
		contextPath := filepath.Join(dataDir, project.ContextPath)
		contextFileName := filepath.Base(project.ContextPath) // e.g., "CLAUDE.md" or "claude.md"
		symlinkPath := filepath.Join(project.ProjectPath, contextFileName)

		os.Remove(symlinkPath) // Remove old symlink

		if err := common.CreateSymlink(contextPath, symlinkPath); err != nil {
			errors = append(errors, fmt.Sprintf("Failed to update %s: %v", project.ContextName, err))
			continue
		}

		// Update global symlinks
		for _, globalName := range project.LinkedGlobals {
			gc := cfg.GetGlobalContext(globalName)
			if gc != nil {
				globalSymlink := filepath.Join(project.ProjectPath, globalName+".md")
				globalPath := filepath.Join(dataDir, gc.Path)

				os.Remove(globalSymlink)
				if err := common.CreateSymlink(globalPath, globalSymlink); err != nil {
					errors = append(errors, fmt.Sprintf("Failed to update global %s for %s: %v", globalName, project.ContextName, err))
				}
			}
		}
	}

	// Update ticket symlinks
	for _, ticket := range cfg.Tickets.Active {
		for _, linkedProj := range ticket.LinkedProjects {
			ticketSymlink := filepath.Join(linkedProj.ProjectPath, fmt.Sprintf("ticket-%s.md", ticket.TicketID))
			ticketPath := filepath.Join(dataDir, "contexts", "_tickets", ticket.TicketID, "ticket.md")

			if common.FileExists(ticketPath) {
				os.Remove(ticketSymlink)
				if err := common.CreateSymlink(ticketPath, ticketSymlink); err != nil {
					errors = append(errors, fmt.Sprintf("Failed to update ticket %s: %v", ticket.TicketID, err))
				}
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("encountered %d errors:\n%s", len(errors), strings.Join(errors, "\n"))
	}

	return nil
}

// copyFile copies a single file
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// Ensure destination directory exists
	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return err
	}

	return destFile.Sync()
}

// copyDir recursively copies a directory
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		return copyFile(path, targetPath)
	})
}

// setupTemplates is deprecated - templates are now embedded in the binary
// User can manually copy templates to ~/.cctx/templates/ for customization
