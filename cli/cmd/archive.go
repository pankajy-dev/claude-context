package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pankaj/claude-context/internal/common"
	"github.com/pankaj/claude-context/internal/config"
	"github.com/spf13/cobra"
)

var archiveCmd = &cobra.Command{
	Use:   "archive <pattern> [pattern...]",
	Short: "Archive files from the current directory",
	Long: `Archive files from the current directory by name or glob pattern.

Files are moved to ~/.cctx/contexts/_archived/<date>_<dirname>/. If a file's
name matches an existing ticket context (e.g. CBP-1.md matches ticket CBP-1),
it is archived into that ticket's existing archive directory instead.

Examples:
  cctx archive "v4-*.md"
  cctx archive old-design.md notes.md
  cctx archive "*.md" --dry-run`,
	Args: cobra.MinimumNArgs(1),
	RunE: runArchive,
}

func init() {
	rootCmd.AddCommand(archiveCmd)
}

func runArchive(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	dataDir := GetDataDirOrExit()
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Expand all patterns against cwd
	matched := map[string]struct{}{}
	for _, pattern := range args {
		// If pattern has no path separator, glob inside cwd
		if !filepath.IsAbs(pattern) && !strings.ContainsRune(pattern, filepath.Separator) {
			pattern = filepath.Join(cwd, pattern)
		}
		hits, err := filepath.Glob(pattern)
		if err != nil {
			return fmt.Errorf("invalid pattern %q: %w", pattern, err)
		}
		for _, h := range hits {
			info, err := os.Lstat(h)
			if err != nil || info.IsDir() {
				continue
			}
			matched[h] = struct{}{}
		}
	}

	if len(matched) == 0 {
		infoMsg("No files matched the given pattern(s)")
		return nil
	}

	// Build ordered list for display
	files := make([]string, 0, len(matched))
	for f := range matched {
		files = append(files, f)
	}
	// stable sort
	sortStrings(files)

	fmt.Println()
	infoMsg(fmt.Sprintf("Found %d file(s) to archive:", len(files)))
	fmt.Println()
	for i, f := range files {
		info, _ := os.Lstat(f)
		size := ""
		if info != nil {
			size = humanSize(info.Size())
		}
		fmt.Printf("  %d. %-40s %s\n", i+1, filepath.Base(f), size)
	}
	fmt.Println()

	if dryRun {
		for _, f := range files {
			dest := resolveArchiveDir(cfgMgr, cfg, cwd, filepath.Base(f))
			dryRunMsg(fmt.Sprintf("Would archive %s → %s", filepath.Base(f), dest))
		}
		return nil
	}

	// Prompt: all / none / comma-range selection
	fmt.Printf("Archive all? [Y/n/select]: ")
	var input string
	fmt.Scanln(&input)
	input = strings.TrimSpace(input)
	fmt.Println()

	var toArchive []string
	switch strings.ToLower(input) {
	case "", "y", "yes":
		toArchive = files
	case "n", "no", "none":
		infoMsg("Operation cancelled")
		return nil
	default:
		// Parse selection like "1,3-5,7"
		selected, err := parseSelection(input, len(files))
		if err != nil {
			return fmt.Errorf("invalid selection: %w", err)
		}
		for _, idx := range selected {
			toArchive = append(toArchive, files[idx])
		}
	}

	if len(toArchive) == 0 {
		infoMsg("Nothing selected")
		return nil
	}

	archivedCount := 0
	for _, src := range toArchive {
		name := filepath.Base(src)
		destDir := resolveArchiveDir(cfgMgr, cfg, cwd, name)

		if err := common.EnsureDir(destDir); err != nil {
			warningMsg(fmt.Sprintf("Failed to create archive dir for %s: %v", name, err))
			continue
		}

		dest := filepath.Join(destDir, name)
		if err := common.CopyFile(src, dest); err != nil {
			warningMsg(fmt.Sprintf("Failed to archive %s: %v", name, err))
			continue
		}
		if err := os.Remove(src); err != nil {
			warningMsg(fmt.Sprintf("Archived but failed to remove original %s: %v", name, err))
		} else {
			successMsg(fmt.Sprintf("Archived %s → %s", name, destDir))
			archivedCount++
		}
	}

	fmt.Println()
	successMsg(fmt.Sprintf("Archived %d file(s)", archivedCount))
	return nil
}

// resolveArchiveDir determines where to archive a file.
// If the filename stem matches an existing ticket archive, use that directory.
// Otherwise use _archived/<date>_<dirname>.
func resolveArchiveDir(cfgMgr *config.Manager, cfg *config.Config, cwd, filename string) string {
	stem := strings.TrimSuffix(filename, filepath.Ext(filename))

	// Check archived tickets first
	for _, t := range cfg.Tickets.Archived {
		if strings.EqualFold(t.TicketID, stem) && t.ArchivedPath != "" {
			candidate := filepath.Join(cfgMgr.GetRepoRoot(), t.ArchivedPath)
			if common.DirExists(candidate) {
				return candidate
			}
		}
	}

	// Check active tickets that have been completed (may have archivepath set)
	for _, t := range cfg.Tickets.Active {
		if strings.EqualFold(t.TicketID, stem) && t.ArchivedPath != "" {
			candidate := filepath.Join(cfgMgr.GetRepoRoot(), t.ArchivedPath)
			if common.DirExists(candidate) {
				return candidate
			}
		}
	}

	// Default: _archived/<date>_<dirname>
	dirname := filepath.Base(cwd)
	return filepath.Join(cfgMgr.GetContextsPath(), "_archived",
		fmt.Sprintf("%s_%s", time.Now().Format("2006-01-02"), dirname))
}

func parseSelection(input string, max int) ([]int, error) {
	selected := map[int]struct{}{}
	for _, part := range strings.Split(input, ",") {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			var lo, hi int
			if _, err := fmt.Sscanf(strings.TrimSpace(bounds[0]), "%d", &lo); err != nil {
				return nil, fmt.Errorf("bad range %q", part)
			}
			if _, err := fmt.Sscanf(strings.TrimSpace(bounds[1]), "%d", &hi); err != nil {
				return nil, fmt.Errorf("bad range %q", part)
			}
			for i := lo; i <= hi; i++ {
				if i >= 1 && i <= max {
					selected[i-1] = struct{}{}
				}
			}
		} else {
			var n int
			if _, err := fmt.Sscanf(part, "%d", &n); err != nil {
				return nil, fmt.Errorf("bad value %q", part)
			}
			if n >= 1 && n <= max {
				selected[n-1] = struct{}{}
			}
		}
	}
	out := make([]int, 0, len(selected))
	for i := range selected {
		out = append(out, i)
	}
	sortInts(out)
	return out, nil
}

func humanSize(b int64) string {
	switch {
	case b >= 1024*1024:
		return fmt.Sprintf("%.1fMB", float64(b)/1024/1024)
	case b >= 1024:
		return fmt.Sprintf("%.1fKB", float64(b)/1024)
	default:
		return fmt.Sprintf("%dB", b)
	}
}

func sortStrings(ss []string) {
	for i := 1; i < len(ss); i++ {
		for j := i; j > 0 && ss[j] < ss[j-1]; j-- {
			ss[j], ss[j-1] = ss[j-1], ss[j]
		}
	}
}

func sortInts(ns []int) {
	for i := 1; i < len(ns); i++ {
		for j := i; j > 0 && ns[j] < ns[j-1]; j-- {
			ns[j], ns[j-1] = ns[j-1], ns[j]
		}
	}
}
