package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pankaj/claude-context/internal/config"
	"github.com/spf13/cobra"
)

var archiveCmd = &cobra.Command{
	Use:   "archive [<ticket-id> | <path|glob>...]",
	Short: "Archive a ticket workspace, a directory, or files",
	Long: `Archive into ~/.cctx/contexts/_archived/. Every run creates a new archive
directory; nothing already archived is ever modified.

  cctx archive CBP-123          Archive ticket CBP-123's .cctx/CBP-123/ workspace
                                (an active ticket is marked completed first)
  cctx archive                  Same, with the ticket ID taken from the git branch
  cctx archive specs/my-feature Archive any directory, e.g. one a skill created
  cctx archive "v4-*.md" a.md   Archive files by name or glob
  cctx archive --all-active     Archive every active ticket (-p to limit to a project)
  cctx archive --list [--all]   List archives from this repo (or everywhere)

Files are copied, verified, recorded in config and only then deleted. Anything
git-tracked, inside the cctx data directory, or containing the current
directory is refused.`,
	Args: cobra.ArbitraryArgs,
	RunE: runArchive,
}

var (
	archiveForce     bool
	archiveAllActive bool
	archiveList      bool
	archiveListAll   bool
	archiveLimit     int
	archiveCommits   string
	archivePRs       string
)

func init() {
	rootCmd.AddCommand(archiveCmd)
	archiveCmd.Flags().BoolVarP(&archiveForce, "force", "f", false, "Skip confirmation prompts")
	archiveCmd.Flags().BoolVar(&archiveAllActive, "all-active", false, "Archive every active ticket")
	archiveCmd.Flags().BoolVar(&archiveList, "list", false, "List archives for the current repo")
	archiveCmd.Flags().BoolVar(&archiveListAll, "all", false, "With --list: show archives from everywhere")
	archiveCmd.Flags().IntVar(&archiveLimit, "limit", 20, "With --list: maximum rows (0 = no limit)")
	archiveCmd.Flags().StringVar(&archiveCommits, "commits", "", "Comma-separated commit hashes (when completing a ticket)")
	archiveCmd.Flags().StringVar(&archivePRs, "prs", "", "Comma-separated PR numbers (when completing a ticket)")
}

func validateArchiveFlags(cmd *cobra.Command, args []string) error {
	changed := func(name string) bool { return cmd.Flags().Changed(name) }
	completionFlags := archiveCommits != "" || archivePRs != ""
	switch {
	case archiveList && (len(args) > 0 || archiveAllActive || completionFlags):
		return fmt.Errorf("--list cannot be combined with arguments, --all-active, --commits or --prs")
	case !archiveList && (archiveListAll || changed("limit")):
		return fmt.Errorf("--all and --limit only apply to --list")
	case archiveLimit < 0:
		return fmt.Errorf("--limit must be 0 or more")
	case archiveAllActive && len(args) > 0:
		return fmt.Errorf("--all-active takes no arguments")
	case archiveAllActive && completionFlags:
		return fmt.Errorf("--commits/--prs cannot be used with --all-active")
	case ticketFlag != "" && (len(args) > 0 || archiveList || archiveAllActive):
		return fmt.Errorf("-t cannot be combined with arguments, --list or --all-active")
	}
	return nil
}

func runArchive(cmd *cobra.Command, args []string) error {
	if err := validateArchiveFlags(cmd, args); err != nil {
		return err
	}

	dataDir := GetDataDirOrExit()
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	switch {
	case archiveList:
		return runArchiveList(cfgMgr)
	case archiveAllActive:
		return runArchiveAllActive(cfgMgr)
	case len(args) == 0:
		id := resolveTicketID(cfg)
		if id == "" || cfg.GetTicket(id, true) == nil {
			return fmt.Errorf("no ticket ID detected; pass a ticket ID, path or glob")
		}
		infoMsg(fmt.Sprintf("Auto-detected ticket ID: %s", id))
		return archiveTicket(cfgMgr, id, archiveTicketOpts())
	}

	for _, arg := range args {
		if cfg.GetTicket(arg, true) != nil {
			if len(args) > 1 {
				return fmt.Errorf("archive one ticket at a time (%s is a ticket ID)", arg)
			}
			return archiveTicket(cfgMgr, arg, archiveTicketOpts())
		}
	}
	if archiveCommits != "" || archivePRs != "" {
		return fmt.Errorf("--commits/--prs are only valid when archiving a ticket")
	}
	return archivePaths(cfgMgr, cfg, args)
}

func archiveTicketOpts() ticketArchiveOpts {
	return ticketArchiveOpts{force: archiveForce, commits: archiveCommits, prs: archivePRs}
}

type pendingItem struct {
	path     string
	isDir    bool
	fromGlob bool
}

type archiveGroup struct {
	name     string
	ticketID string
	source   string
	items    []pendingItem
}

// archivePaths handles path and glob arguments. Each directory gets its own
// archive; files are grouped by the ticket ID in their name, else by parent dir.
func archivePaths(cfgMgr *config.Manager, cfg *config.Config, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	ticketIDs := cfg.AllTicketIDs()

	var pending []pendingItem
	seen := map[string]bool{}
	add := func(p pendingItem) {
		if !seen[p.path] {
			seen[p.path] = true
			pending = append(pending, p)
		}
	}
	for _, arg := range args {
		// Clean drops a trailing slash, which would make Lstat follow a symlink
		p := filepath.Clean(arg)
		if !filepath.IsAbs(p) {
			p = filepath.Join(cwd, p)
		}
		if info, err := os.Lstat(p); err == nil {
			add(pendingItem{path: p, isDir: info.IsDir()})
			continue
		}
		matches, err := matchArchiveFiles(cwd, []string{arg})
		if err != nil {
			return err
		}
		if !strings.ContainsAny(arg, "*?[") && !strings.ContainsRune(arg, filepath.Separator) {
			matches = filterIDCollisions(matches, arg, ticketIDs)
		}
		for _, m := range matches {
			if info, err := os.Lstat(m); err == nil && info.Mode()&os.ModeSymlink != 0 {
				warningMsg(fmt.Sprintf("skipped symlink %s", filepath.Base(m)))
				continue
			}
			add(pendingItem{path: m, fromGlob: true})
		}
	}

	if len(pending) == 0 {
		infoMsg("No files matched the given pattern(s)")
		return nil
	}

	fmt.Println()
	infoMsg(fmt.Sprintf("Found %d item(s) to archive:", len(pending)))
	fmt.Println()
	for i, p := range pending {
		label := filepath.Base(p.path)
		detail := ""
		if p.isDir {
			label += "/"
			detail = fmt.Sprintf("%d file(s)", countArchiveEntries(p.path))
		} else if info, err := os.Lstat(p.path); err == nil {
			detail = humanSize(info.Size())
		}
		fmt.Printf("  %d. %-40s %s\n", i+1, label, detail)
	}
	fmt.Println()

	selected := pending
	if !dryRun && !archiveForce {
		fmt.Printf("Archive all? [Y/n/select]: ")
		var input string
		fmt.Scanln(&input)
		input = strings.TrimSpace(input)
		fmt.Println()
		switch strings.ToLower(input) {
		case "", "y", "yes":
		case "n", "no", "none":
			infoMsg("Operation cancelled")
			return nil
		default:
			idx, err := parseSelection(input, len(pending))
			if err != nil {
				return fmt.Errorf("invalid selection: %w", err)
			}
			selected = nil
			for _, i := range idx {
				selected = append(selected, pending[i])
			}
		}
	}
	if len(selected) == 0 {
		infoMsg("Nothing selected")
		return nil
	}
	// docs and "docs/*.md" together would archive docs/a.md twice
	var deduped []pendingItem
	for _, p := range selected {
		nested := false
		for _, d := range selected {
			if d.isDir && d.path != p.path && isWithin(canon(p.path), canon(d.path)) {
				nested = true
				break
			}
		}
		if !nested {
			deduped = append(deduped, p)
		}
	}
	selected = deduped

	type plannedGroup struct {
		group archiveGroup
		items []archiveItem
		kind  string
	}
	var plan []plannedGroup
	var all []archiveItem
	for gi, g := range groupArchiveItems(selected, ticketIDs, cwd) {
		pg := plannedGroup{group: g, kind: "path"}
		for _, p := range g.items {
			rel := filepath.Base(p.path)
			if p.isDir {
				rel = "."
			}
			if p.fromGlob {
				pg.kind = "glob"
			}
			pg.items = append(pg.items, archiveItem{Src: p.path, RelDest: rel})
			// groups land in different dirs, so only collisions within a group count
			all = append(all, archiveItem{Src: p.path, RelDest: filepath.Join(fmt.Sprint(gi), rel)})
		}
		plan = append(plan, pg)
	}
	// Check everything up front so a refused item can't leave a half-done run
	if err := archiveRefusals(cfg, cfgMgr.GetRepoRoot(), all, archiveOpts{}); err != nil {
		return err
	}

	for _, pg := range plan {
		final, err := archiveItems(cfgMgr, pg.items, archiveOpts{
			Name:   pg.group.name,
			Record: config.ArchiveRecord{Kind: pg.kind, TicketID: pg.group.ticketID, SourcePath: canon(pg.group.source)},
		})
		if err != nil {
			return err
		}
		if final != "" {
			successMsg(fmt.Sprintf("Archived %d item(s) → %s", len(pg.items), final))
		}
	}
	return nil
}

// ticketForFile returns the longest ticket ID the file name belongs to.
func ticketForFile(name string, ticketIDs []string) string {
	best := ""
	for _, id := range ticketIDs {
		if len(id) > len(best) && IsTicketFileName(name, id) {
			best = id
		}
	}
	return best
}

func groupArchiveItems(items []pendingItem, ticketIDs []string, cwd string) []archiveGroup {
	var groups []archiveGroup
	byKey := map[string]int{}
	fileTickets := map[string]bool{}
	otherFiles := false

	for _, p := range items {
		if !p.isDir && filepath.Base(p.path) != "SESSIONS.md" {
			if id := ticketForFile(filepath.Base(p.path), ticketIDs); id != "" {
				fileTickets[id] = true
			} else {
				otherFiles = true
			}
		}
	}
	// A SESSIONS.md archived alongside one ticket's files belongs to that ticket.
	sessionsTicket := ""
	if len(fileTickets) == 1 && !otherFiles {
		for id := range fileTickets {
			sessionsTicket = id
		}
	}

	addTo := func(key string, g archiveGroup, p pendingItem) {
		if i, ok := byKey[key]; ok {
			groups[i].items = append(groups[i].items, p)
			return
		}
		g.items = []pendingItem{p}
		byKey[key] = len(groups)
		groups = append(groups, g)
	}

	for _, p := range items {
		name := filepath.Base(p.path)
		switch {
		case p.isDir:
			groups = append(groups, archiveGroup{name: name, source: p.path, items: []pendingItem{p}})
		case name == "SESSIONS.md" && sessionsTicket != "":
			addTo("ticket:"+sessionsTicket, archiveGroup{name: sessionsTicket, ticketID: sessionsTicket, source: filepath.Dir(p.path)}, p)
		default:
			if id := ticketForFile(name, ticketIDs); id != "" {
				addTo("ticket:"+id, archiveGroup{name: id, ticketID: id, source: filepath.Dir(p.path)}, p)
			} else {
				dir := filepath.Dir(p.path)
				addTo("dir:"+dir, archiveGroup{name: filepath.Base(cwd), source: dir}, p)
			}
		}
	}
	return groups
}

// matchArchiveFiles expands patterns against cwd, keeping the order they were
// given in. filepath.Glob returns sorted matches, so a single "*.md" still
// lists alphabetically, while an explicit list keeps its caller's ordering.
//
// A bare ticket ID like "SECO-5669" has no extension and no glob
// metacharacters, so it never literally matches SECO-5669.md via Glob. For
// those, fall back to whole-word ticket file matching so `cctx archive
// SECO-5669` behaves like `cctx archive SECO-5669.md` and also picks up
// companions such as prefix-SECO-5669-suffix.md.
func matchArchiveFiles(cwd string, patterns []string) ([]string, error) {
	files := []string{}
	seen := map[string]struct{}{}
	addFile := func(h string) {
		info, err := os.Lstat(h)
		if err != nil || info.IsDir() {
			return
		}
		if _, dup := seen[h]; dup {
			return
		}
		seen[h] = struct{}{}
		files = append(files, h)
	}

	for _, arg := range patterns {
		pattern := arg
		relative := !filepath.IsAbs(pattern) && !strings.ContainsRune(pattern, filepath.Separator)
		if relative {
			pattern = filepath.Join(cwd, pattern)
		}
		hits, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %q: %w", pattern, err)
		}
		for _, h := range hits {
			addFile(h)
		}

		if len(hits) == 0 && relative && !strings.ContainsAny(arg, "*?[") {
			for _, h := range FindTicketFiles(cwd, arg) {
				addFile(h)
			}
		}
	}

	return files, nil
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
