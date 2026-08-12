package cmd

import (
	"github.com/spf13/cobra"
)

var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Setup, maintenance and context management",
	Long: `Administrative commands for cctx itself.

One-time setup and maintenance:
  init        Initialize the data directory
  completion  Generate a shell completion script
  home        Show or open the data directory
  verify      Check health of managed projects and symlinks
  cleanup     Remove orphaned symlinks and stale data
  reset       Reset cctx to a clean state

Project context management (the original claude.md symlink feature):
  sync      Import an existing claude.md from a project
  link      Link a project to the context manager
  unlink    Unlink a project
  list      List managed projects
  global    Manage global context files

Day-to-day ticket work lives under 'cctx ticket'.`,
}

func init() {
	rootCmd.AddCommand(adminCmd)
}
