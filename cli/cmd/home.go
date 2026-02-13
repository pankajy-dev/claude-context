package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	homeOpen   bool
	homeList   bool
	homeShell  bool
)

var homeCmd = &cobra.Command{
	Use:   "home",
	Short: "Show or open the cctx data directory",
	Long: `Show or open the cctx data directory where all contexts are stored.

By default, prints the path to the data directory.

💡 TIP: Source cctx-shell-functions.sh to make 'cctx home' actually cd!
   Add to ~/.bashrc or ~/.zshrc:
     source /path/to/cctx-shell-functions.sh

Common usage patterns:
  cctx home                      # Print path: /Users/you/.cctx
  cd $(cctx home)                # Navigate to data directory
  ls $(cctx home)/contexts       # List all contexts
  cctx home --list               # Show directory tree
  cctx home --open               # Open in Finder/Explorer
  cctx home --shell              # Start shell in data directory

Useful for manual file management:
  cd $(cctx home)/contexts/_tickets/TICKET-123
  vim $(cctx home)/contexts/my-project/claude.md
  rm -rf $(cctx home)/contexts/_archived/old-ticket`,
	RunE: runHome,
}

func init() {
	rootCmd.AddCommand(homeCmd)
	homeCmd.Flags().BoolVarP(&homeOpen, "open", "o", false, "Open data directory in file manager")
	homeCmd.Flags().BoolVarP(&homeList, "list", "l", false, "List directory structure")
	homeCmd.Flags().BoolVarP(&homeShell, "shell", "s", false, "Open a new shell in the data directory")
}

func runHome(cmd *cobra.Command, args []string) error {
	dataDir := GetDataDirOrExit()

	// Check if data directory exists
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		return fmt.Errorf("data directory not initialized: %s\nRun 'cctx init' first", dataDir)
	}

	// Handle --shell flag
	if homeShell {
		return openShell(dataDir)
	}

	// Handle --open flag
	if homeOpen {
		return openInFileManager(dataDir)
	}

	// Handle --list flag
	if homeList {
		return listDataDir(dataDir)
	}

	// Default: just print the path
	fmt.Println(dataDir)
	return nil
}

func openInFileManager(path string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "linux":
		// Try common file managers
		for _, fm := range []string{"xdg-open", "nautilus", "dolphin", "thunar"} {
			if _, err := exec.LookPath(fm); err == nil {
				cmd = exec.Command(fm, path)
				break
			}
		}
		if cmd == nil {
			return fmt.Errorf("no suitable file manager found")
		}
	case "windows":
		cmd = exec.Command("explorer", path)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to open file manager: %w", err)
	}

	successMsg(fmt.Sprintf("Opened %s in file manager", path))
	return nil
}

func openShell(path string) error {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	infoMsg(fmt.Sprintf("Opening shell in: %s", path))
	infoMsg("Type 'exit' to return to your previous shell")
	fmt.Println()

	cmd := exec.Command(shell)
	cmd.Dir = path
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start shell: %w", err)
	}

	return nil
}

func listDataDir(path string) error {
	successMsg(fmt.Sprintf("Data directory: %s", path))
	fmt.Println()

	// Use tree if available, otherwise use ls
	if _, err := exec.LookPath("tree"); err == nil {
		cmd := exec.Command("tree", "-L", "3", "-a", "--dirsfirst", path)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	// Fallback to basic ls
	cmd := exec.Command("ls", "-lah", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}