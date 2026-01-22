package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/pankaj/claude-context/internal/common"
	"github.com/pankaj/claude-context/internal/config"
	"github.com/spf13/cobra"
)

var globalCmd = &cobra.Command{
	Use:   "global",
	Short: "Manage global context files",
	Long:  `Manage global context files shared across all projects.`,
}

var (
	globalTitle       string
	globalDescription string
	globalAddExisting bool
	globalKeepFile    bool
	globalLinkAll     bool
)

// global init subcommand
var globalInitCmd = &cobra.Command{
	Use:   "init <name>",
	Short: "Create a new global context file",
	Long: `Create a new global context file shared across all projects.

Global context files contain preferences and standards that apply to all your
projects (e.g., coding standards, architecture patterns, script guidelines).`,
	Args: cobra.ExactArgs(1),
	RunE: runGlobalInit,
}

// global list subcommand
var globalListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all global context files",
	Long:  `List all global context files and their status (enabled/disabled).`,
	RunE:  runGlobalList,
}

// global enable subcommand
var globalEnableCmd = &cobra.Command{
	Use:   "enable <name>",
	Short: "Enable a global context",
	Long:  `Enable a global context. It will be linked to new projects automatically.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runGlobalEnable,
}

// global disable subcommand
var globalDisableCmd = &cobra.Command{
	Use:   "disable <name>",
	Short: "Disable a global context",
	Long:  `Disable a global context. It won't be linked to new projects.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runGlobalDisable,
}

// global remove subcommand
var globalRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a global context",
	Long: `Remove a global context.

Requires confirmation before proceeding.`,
	Args: cobra.ExactArgs(1),
	RunE: runGlobalRemove,
}

// global link subcommand
var globalLinkCmd = &cobra.Command{
	Use:   "link <name> [<project1> <project2>...]",
	Short: "Link a global context to specific projects",
	Long: `Link a global context to one or more projects.

Creates symlinks in the project directories and tracks the linkage in config.json.

You can specify projects in multiple ways:
  - As arguments: cctx global link script api-gateway frontend
  - Using --project flag: cctx -p api-gateway global link script
  - Using CCTX_PROJECT env: export CCTX_PROJECT=api-gateway && cctx global link script
  - Use --all to link to all projects: cctx global link script --all`,
	Args: cobra.MinimumNArgs(1),
	RunE: runGlobalLink,
}

// global unlink subcommand
var globalUnlinkCmd = &cobra.Command{
	Use:   "unlink <name> [<project1> <project2>...]",
	Short: "Unlink a global context from specific projects",
	Long: `Unlink a global context from one or more projects.

Removes symlinks from the project directories and updates config.json.

You can specify projects in multiple ways:
  - As arguments: cctx global unlink script api-gateway frontend
  - Using --project flag: cctx -p api-gateway global unlink script
  - Using CCTX_PROJECT env: export CCTX_PROJECT=api-gateway && cctx global unlink script`,
	Args: cobra.MinimumNArgs(1),
	RunE: runGlobalUnlink,
}

// global show subcommand
var globalShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show which projects use a global context",
	Long:  `Display all projects that have a specific global context linked.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runGlobalShow,
}

func init() {
	rootCmd.AddCommand(globalCmd)

	// global init
	globalCmd.AddCommand(globalInitCmd)
	globalInitCmd.Flags().StringVar(&globalTitle, "title", "", "Human-readable title")
	globalInitCmd.Flags().StringVar(&globalDescription, "description", "", "Description of the global context")
	globalInitCmd.Flags().BoolVar(&globalAddExisting, "add-to-existing", false, "Add to existing projects immediately")

	// global list
	globalCmd.AddCommand(globalListCmd)

	// global enable
	globalCmd.AddCommand(globalEnableCmd)

	// global disable
	globalCmd.AddCommand(globalDisableCmd)

	// global remove
	globalCmd.AddCommand(globalRemoveCmd)
	globalRemoveCmd.Flags().BoolVar(&globalKeepFile, "keep-file", false, "Keep file but remove from config")

	// global link
	globalCmd.AddCommand(globalLinkCmd)
	globalLinkCmd.Flags().BoolVar(&globalLinkAll, "all", false, "Link to all managed projects")

	// global unlink
	globalCmd.AddCommand(globalUnlinkCmd)

	// global show
	globalCmd.AddCommand(globalShowCmd)
}

func runGlobalInit(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Get data directory
	dataDir := GetDataDirOrExit()

	// Load config
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if global context already exists
	for _, gc := range cfg.GlobalContexts {
		if gc.Name == name {
			return fmt.Errorf("global context already exists: %s", name)
		}
	}

	globalDir := filepath.Join(cfgMgr.GetContextsPath(), "_global")
	globalFile := filepath.Join(globalDir, name+".md")

	infoMsg(fmt.Sprintf("Creating global context: %s", name))
	if globalAddExisting {
		infoMsg(fmt.Sprintf("Will link to %d existing projects", len(cfg.ManagedProjects)))
	}
	fmt.Println()

	if dryRun {
		dryRunMsg(fmt.Sprintf("Would create directory: %s", globalDir))
		dryRunMsg(fmt.Sprintf("Would create file: %s", globalFile))
		if globalAddExisting {
			dryRunMsg("Would link to existing projects")
		}
	} else {
		// Create _global directory
		if err := common.EnsureDir(globalDir); err != nil {
			return fmt.Errorf("failed to create global directory: %w", err)
		}

		// Create global context file from template
		template := fmt.Sprintf("# Global Context: %s\n\n", name)
		if globalTitle != "" {
			template = fmt.Sprintf("# %s\n\n", globalTitle)
		}
		if globalDescription != "" {
			template += fmt.Sprintf("## Description\n\n%s\n\n", globalDescription)
		}
		template += "## Standards\n\n[Add your standards and preferences here]\n\n"

		if err := os.WriteFile(globalFile, []byte(template), 0644); err != nil {
			return fmt.Errorf("failed to create global context file: %w", err)
		}
		successMsg("Created global context file")

		// Add to config
		gc := config.GlobalContext{
			Name:        name,
			Description: globalDescription,
			Path:        "contexts/_global/" + name + ".md",
			Enabled:     true,
		}

		cfg.GlobalContexts = append(cfg.GlobalContexts, gc)

		if err := cfgMgr.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		successMsg("Updated configuration")

		// Link to existing projects if requested
		if globalAddExisting {
			linkedCount := 0
			for _, project := range cfg.ManagedProjects {
				projectGlobal := filepath.Join(project.ProjectPath, name+".md")

				if err := common.CreateSymlink(globalFile, projectGlobal); err != nil {
					warningMsg(fmt.Sprintf("Failed to link to %s: %v", project.ContextName, err))
				} else {
					successMsg(fmt.Sprintf("Linked to %s", project.ContextName))
					linkedCount++
				}
			}
			infoMsg(fmt.Sprintf("Linked to %d projects", linkedCount))
		}
	}

	// Git commit removed (no longer tracking in git)

	if !dryRun {
		fmt.Println()
		successMsg(fmt.Sprintf("Successfully created global context: %s", name))
		infoMsg(fmt.Sprintf("File: %s", globalFile))
		infoMsg("Edit the file to add your standards and preferences")
	}

	return nil
}

func runGlobalList(cmd *cobra.Command, args []string) error {
	// Get data directory
	dataDir := GetDataDirOrExit()

	// Load config
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if len(cfg.GlobalContexts) == 0 {
		infoMsg("No global contexts found")
		infoMsg("Create one with: cctx global init <name>")
		return nil
	}

	// Print table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	if verbose {
		fmt.Fprintln(w, "NAME\tSTATUS\tDESCRIPTION\t")
	} else {
		fmt.Fprintln(w, "NAME\tSTATUS\t")
	}

	for _, gc := range cfg.GlobalContexts {
		status := "disabled"
		if gc.Enabled {
			status = "enabled"
		}

		description := gc.Description
		if description == "" {
			description = "-"
		}

		if verbose {
			fmt.Fprintf(w, "%s\t%s\t%s\t\n",
				gc.Name,
				status,
				description,
			)
		} else {
			fmt.Fprintf(w, "%s\t%s\t\n",
				gc.Name,
				status,
			)
		}
	}

	w.Flush()

	fmt.Printf("\nTotal: %d global contexts\n", len(cfg.GlobalContexts))

	return nil
}

func runGlobalEnable(cmd *cobra.Command, args []string) error {
	name := normalizeGlobalName(args[0])

	// Get data directory
	dataDir := GetDataDirOrExit()

	// Load config
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Find global context
	var gc *config.GlobalContext
	for i := range cfg.GlobalContexts {
		if cfg.GlobalContexts[i].Name == name {
			gc = &cfg.GlobalContexts[i]
			break
		}
	}

	if gc == nil {
		return fmt.Errorf("global context not found: %s", name)
	}

	if gc.Enabled {
		infoMsg(fmt.Sprintf("Global context '%s' is already enabled", name))
		return nil
	}

	gc.Enabled = true

	if dryRun {
		dryRunMsg(fmt.Sprintf("Would enable global context: %s", name))
		return nil
	}

	// Save config
	if err := cfgMgr.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	successMsg("Updated configuration")

	// Git commit removed (no longer tracking in git)

	fmt.Println()
	successMsg(fmt.Sprintf("Enabled global context: %s", name))
	infoMsg("New projects will automatically get this global context")

	return nil
}

func runGlobalDisable(cmd *cobra.Command, args []string) error {
	name := normalizeGlobalName(args[0])

	// Get data directory
	dataDir := GetDataDirOrExit()

	// Load config
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Find global context
	var gc *config.GlobalContext
	for i := range cfg.GlobalContexts {
		if cfg.GlobalContexts[i].Name == name {
			gc = &cfg.GlobalContexts[i]
			break
		}
	}

	if gc == nil {
		return fmt.Errorf("global context not found: %s", name)
	}

	if !gc.Enabled {
		infoMsg(fmt.Sprintf("Global context '%s' is already disabled", name))
		return nil
	}

	gc.Enabled = false

	if dryRun {
		dryRunMsg(fmt.Sprintf("Would disable global context: %s", name))
		return nil
	}

	// Save config
	if err := cfgMgr.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	successMsg("Updated configuration")

	// Git commit removed (no longer tracking in git)

	fmt.Println()
	successMsg(fmt.Sprintf("Disabled global context: %s", name))
	infoMsg("New projects will not get this global context")
	infoMsg("Existing projects keep their symlinks (use verify to check)")

	return nil
}

func runGlobalRemove(cmd *cobra.Command, args []string) error {
	name := normalizeGlobalName(args[0])

	// Get data directory
	dataDir := GetDataDirOrExit()

	// Load config
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Find global context
	gcIndex := -1
	var gc *config.GlobalContext
	for i := range cfg.GlobalContexts {
		if cfg.GlobalContexts[i].Name == name {
			gcIndex = i
			gc = &cfg.GlobalContexts[i]
			break
		}
	}

	if gc == nil {
		return fmt.Errorf("global context not found: %s", name)
	}

	globalFile := filepath.Join(dataDir, gc.Path)

	infoMsg(fmt.Sprintf("Removing global context: %s", name))
	if !globalKeepFile {
		warningMsg("File will be deleted")
	}
	fmt.Println()

	// Confirmation required
	if !dryRun {
		if !common.Confirm("Proceed with removal?", false) {
			infoMsg("Operation cancelled")
			return nil
		}
		fmt.Println()
	}

	// Remove from config
	cfg.GlobalContexts = append(cfg.GlobalContexts[:gcIndex], cfg.GlobalContexts[gcIndex+1:]...)

	// Delete file if requested
	if !globalKeepFile {
		if dryRun {
			dryRunMsg(fmt.Sprintf("Would delete file: %s", globalFile))
		} else {
			if common.FileExists(globalFile) {
				if err := os.Remove(globalFile); err != nil {
					warningMsg(fmt.Sprintf("Failed to delete file: %v", err))
				} else {
					successMsg("Deleted global context file")
				}
			}
		}
	} else {
		infoMsg("Keeping global context file")
	}

	if dryRun {
		dryRunMsg("Would remove from configuration")
	} else {
		if err := cfgMgr.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		successMsg("Updated configuration")
	}

	// Git commit removed (no longer tracking in git)

	if !dryRun {
		fmt.Println()
		successMsg(fmt.Sprintf("Successfully removed global context: %s", name))
		infoMsg("Existing symlinks in projects may still exist")
		infoMsg("Run 'cctx verify' to check")
	}

	return nil
}

func runGlobalLink(cmd *cobra.Command, args []string) error {
	globalName := normalizeGlobalName(args[0])
	var projectNames []string

	// Get data directory
	dataDir := GetDataDirOrExit()

	// Load config
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Find global context
	gc := cfg.GetGlobalContext(globalName)
	if gc == nil {
		return fmt.Errorf("global context not found: %s", globalName)
	}

	globalFile := filepath.Join(dataDir, gc.Path)
	if !common.FileExists(globalFile) {
		return fmt.Errorf("global context file does not exist: %s", globalFile)
	}

	// Determine which projects to link
	if globalLinkAll {
		// Link to all projects
		for _, p := range cfg.ManagedProjects {
			projectNames = append(projectNames, p.ContextName)
		}
		infoMsg(fmt.Sprintf("Linking '%s' to all %d projects", globalName, len(projectNames)))
	} else if len(args) > 1 {
		// Projects specified as arguments
		for _, projectArg := range args[1:] {
			resolvedName, err := resolveProjectName(projectArg, cfg)
			if err != nil {
				return err
			}
			projectNames = append(projectNames, resolvedName)
		}
		infoMsg(fmt.Sprintf("Linking '%s' to %d projects", globalName, len(projectNames)))
	} else {
		// Try to get from --project flag or env var
		projectName, err := GetProjectContext(dataDir)
		if err != nil {
			return err
		}
		if projectName == "" {
			return fmt.Errorf("no projects specified. Use --project flag, set CCTX_PROJECT, provide project names as arguments, or use --all")
		}
		projectNames = []string{projectName}
		infoMsg(fmt.Sprintf("Linking '%s' to project: %s", globalName, projectName))
	}

	fmt.Println()

	linkedCount := 0
	alreadyLinkedCount := 0

	for _, projectName := range projectNames {
		project := cfg.GetProject(projectName)
		if project == nil {
			warningMsg(fmt.Sprintf("Project not found: %s", projectName))
			continue
		}

		// Check if already linked
		alreadyLinked := false
		for _, lg := range project.LinkedGlobals {
			if lg == globalName {
				alreadyLinked = true
				break
			}
		}

		if alreadyLinked {
			infoMsg(fmt.Sprintf("%s: already linked", projectName))
			alreadyLinkedCount++
			continue
		}

		// Create symlink
		projectGlobal := filepath.Join(project.ProjectPath, globalName+".md")

		if dryRun {
			dryRunMsg(fmt.Sprintf("%s: would create symlink", projectName))
		} else {
			if err := common.CreateSymlink(globalFile, projectGlobal); err != nil {
				warningMsg(fmt.Sprintf("%s: failed to create symlink: %v", projectName, err))
				continue
			}

			// Add to project's linked_globals
			project.LinkedGlobals = append(project.LinkedGlobals, globalName)
			successMsg(fmt.Sprintf("%s: linked", projectName))
			linkedCount++
		}
	}

	if !dryRun && linkedCount > 0 {
		// Save config
		if err := cfgMgr.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		successMsg("Updated configuration")
	}

	// Git commit removed (no longer tracking in git)

	if !dryRun {
		fmt.Println()
		successMsg(fmt.Sprintf("Linked to %d projects", linkedCount))
		if alreadyLinkedCount > 0 {
			infoMsg(fmt.Sprintf("%d projects were already linked", alreadyLinkedCount))
		}
	}

	return nil
}

func runGlobalUnlink(cmd *cobra.Command, args []string) error {
	globalName := normalizeGlobalName(args[0])
	var projectNames []string

	// Get data directory
	dataDir := GetDataDirOrExit()

	// Load config
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Find global context
	gc := cfg.GetGlobalContext(globalName)
	if gc == nil {
		return fmt.Errorf("global context not found: %s", globalName)
	}

	// Determine project names
	if len(args) > 1 {
		// Projects specified as arguments
		for _, projectArg := range args[1:] {
			resolvedName, err := resolveProjectName(projectArg, cfg)
			if err != nil {
				return err
			}
			projectNames = append(projectNames, resolvedName)
		}
	} else {
		// Try to get from --project flag or env var
		projectName, err := GetProjectContext(dataDir)
		if err != nil {
			return err
		}
		if projectName == "" {
			return fmt.Errorf("no projects specified. Use --project flag, set CCTX_PROJECT, or provide project names as arguments")
		}
		projectNames = []string{projectName}
	}

	infoMsg(fmt.Sprintf("Unlinking '%s' from %d projects", globalName, len(projectNames)))
	fmt.Println()

	unlinkedCount := 0
	notLinkedCount := 0

	for _, projectName := range projectNames {
		project := cfg.GetProject(projectName)
		if project == nil {
			warningMsg(fmt.Sprintf("Project not found: %s", projectName))
			continue
		}

		// Check if linked
		linkedIndex := -1
		for i, lg := range project.LinkedGlobals {
			if lg == globalName {
				linkedIndex = i
				break
			}
		}

		if linkedIndex == -1 {
			infoMsg(fmt.Sprintf("%s: not linked", projectName))
			notLinkedCount++
			continue
		}

		// Remove symlink
		projectGlobal := filepath.Join(project.ProjectPath, globalName+".md")

		if dryRun {
			dryRunMsg(fmt.Sprintf("%s: would remove symlink", projectName))
		} else {
			if common.FileExists(projectGlobal) || common.IsSymlink(projectGlobal) {
				if err := os.Remove(projectGlobal); err != nil {
					warningMsg(fmt.Sprintf("%s: failed to remove symlink: %v", projectName, err))
					continue
				}
			}

			// Remove from project's linked_globals
			project.LinkedGlobals = append(project.LinkedGlobals[:linkedIndex], project.LinkedGlobals[linkedIndex+1:]...)
			successMsg(fmt.Sprintf("%s: unlinked", projectName))
			unlinkedCount++
		}
	}

	if !dryRun && unlinkedCount > 0 {
		// Save config
		if err := cfgMgr.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		successMsg("Updated configuration")
	}

	// Git commit removed (no longer tracking in git)

	if !dryRun {
		fmt.Println()
		successMsg(fmt.Sprintf("Unlinked from %d projects", unlinkedCount))
		if notLinkedCount > 0 {
			infoMsg(fmt.Sprintf("%d projects were not linked", notLinkedCount))
		}
	}

	return nil
}

func runGlobalShow(cmd *cobra.Command, args []string) error {
	globalName := normalizeGlobalName(args[0])

	// Get data directory
	dataDir := GetDataDirOrExit()

	// Load config
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Find global context
	gc := cfg.GetGlobalContext(globalName)
	if gc == nil {
		return fmt.Errorf("global context not found: %s", globalName)
	}

	// Find all projects using this global context
	var linkedProjects []string
	for _, project := range cfg.ManagedProjects {
		for _, lg := range project.LinkedGlobals {
			if lg == globalName {
				linkedProjects = append(linkedProjects, project.ContextName)
				break
			}
		}
	}

	// Display info
	fmt.Printf("Global Context: %s\n", globalName)
	fmt.Printf("File: %s\n", gc.Path)
	fmt.Printf("Status: ")
	if gc.Enabled {
		fmt.Println("enabled (auto-links to new projects)")
	} else {
		fmt.Println("disabled")
	}
	if gc.Description != "" {
		fmt.Printf("Description: %s\n", gc.Description)
	}
	fmt.Println()

	if len(linkedProjects) == 0 {
		infoMsg("No projects are currently linked to this global context")
	} else {
		fmt.Printf("Linked to %d projects:\n", len(linkedProjects))
		for _, projectName := range linkedProjects {
			fmt.Printf("  - %s\n", projectName)
		}
	}

	return nil
}

// normalizeGlobalName resolves global context name from various formats
func normalizeGlobalName(name string) string {
	// Handle file paths (./script.md, /path/to/script.md, script.md)
	if strings.Contains(name, string(filepath.Separator)) || filepath.Ext(name) == ".md" {
		// Get the base filename
		base := filepath.Base(name)
		// Strip .md extension
		if filepath.Ext(base) == ".md" {
			return base[:len(base)-3]
		}
		return base
	}
	return name
}

// resolveProjectName resolves a project name, handling "." as current directory
func resolveProjectName(projectArg string, cfg *config.Config) (string, error) {
	// If ".", resolve to current directory
	if projectArg == "." {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current directory: %w", err)
		}

		// Normalize the path
		normalizedCwd, err := common.NormalizePath(cwd)
		if err != nil {
			return "", fmt.Errorf("failed to normalize path: %w", err)
		}

		// Find project by path
		for _, project := range cfg.ManagedProjects {
			if project.ProjectPath == normalizedCwd {
				return project.ContextName, nil
			}
		}

		return "", fmt.Errorf("current directory is not a managed project: %s", normalizedCwd)
	}

	// Otherwise, return as-is (will be validated later)
	return projectArg, nil
}
