package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var initSpecsCmd = &cobra.Command{
	Use:   "init-specs",
	Short: "Scaffold mission, tech-stack and roadmap docs in .cctx/",
	Long: `Create the project-wide spec documents in ./.cctx/:

  mission.md     what the project is for and who it serves
  tech-stack.md  what it is built with and what's missing
  roadmap.md     small, ordered phases of work

Each file starts with instructions for Claude to interview you before filling
it in. Existing files are never overwritten.`,
	Args: cobra.NoArgs,
	RunE: runInitSpecs,
}

func init() {
	rootCmd.AddCommand(initSpecsCmd)
}

func runInitSpecs(cmd *cobra.Command, args []string) error {
	dataDir := GetDataDirOrExit()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	ws, err := ensureWorkspaceDir(cwd, dataDir)
	if err != nil {
		return err
	}

	if !dryRun {
		if err := os.MkdirAll(ws, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", ws, err)
		}
	}
	for _, f := range specsTemplateFiles {
		path := filepath.Join(ws, f.file)
		if _, err := os.Lstat(path); err == nil {
			infoMsg(fmt.Sprintf("exists, skipped: %s", path))
			continue
		}
		if dryRun {
			dryRunMsg(fmt.Sprintf("Would create %s", path))
			continue
		}
		if err := writeTemplateFile(path, f.template, "", dataDir); err != nil {
			return fmt.Errorf("failed to write %s: %w", f.file, err)
		}
		successMsg(fmt.Sprintf("Created %s", path))
	}
	return nil
}
