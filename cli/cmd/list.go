package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/pankaj/claude-context/internal/common"
	"github.com/pankaj/claude-context/internal/config"
	"github.com/spf13/cobra"
)

var (
	listBroken bool
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all managed projects",
	Long:  `List all projects managed by Claude Context Manager with their status.`,
	RunE:  runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().BoolVarP(&listBroken, "broken", "b", false, "Show only broken links")
}

func runList(cmd *cobra.Command, args []string) error {
	// Get data directory
	dataDir := GetDataDirOrExit()

	// Load config
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if len(cfg.ManagedProjects) == 0 {
		infoMsg("No managed projects found")
		return nil
	}

	// Create table writer
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Header
	if verbose {
		fmt.Fprintln(w, "CONTEXT\tSTATUS\tPROJECT PATH\tLINKED AT\t")
	} else {
		fmt.Fprintln(w, "CONTEXT\tSTATUS\tPROJECT PATH\t")
	}

	// List projects
	for _, project := range cfg.ManagedProjects {
		// Check if project directory exists
		status := "✓"
		statusText := "OK"

		if !common.DirExists(project.ProjectPath) {
			status = "✗"
			statusText = "BROKEN"
		} else {
			// Check context file (should be concrete file in project)
			contextFileName := filepath.Base(project.ContextPath) // e.g., "CLAUDE.md" or "claude.md"
			claudeMD := filepath.Join(project.ProjectPath, contextFileName)

			if !common.FileExists(claudeMD) {
				status = "✗"
				statusText = "BROKEN"
			} else if common.IsSymlink(claudeMD) {
				// Project file should be concrete, not symlink
				status = "✗"
				statusText = "BROKEN"
			}
		}

		// Skip if filtering for broken links only
		if listBroken && status == "✓" {
			continue
		}

		// Print row
		if verbose {
			fmt.Fprintf(w, "%s\t%s %s\t%s\t%s\t\n",
				project.ContextName,
				status,
				statusText,
				project.ProjectPath,
				project.CreatedAt.Format("2006-01-02"),
			)
		} else {
			fmt.Fprintf(w, "%s\t%s %s\t%s\t\n",
				project.ContextName,
				status,
				statusText,
				project.ProjectPath,
			)
		}
	}

	w.Flush()

	// Summary
	fmt.Printf("\nTotal: %d projects\n", len(cfg.ManagedProjects))

	return nil
}
