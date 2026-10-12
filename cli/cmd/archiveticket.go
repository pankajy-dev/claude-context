package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pankaj/claude-context/internal/common"
	"github.com/pankaj/claude-context/internal/config"
)

type ticketArchiveOpts struct {
	approveCompletion bool // skip the "is active" prompt (ticket complete, --all-active)
	completeAbandoned bool // ticket complete also completes abandoned tickets
	force             bool
	commits           string
	prs               string
}

type completionInfo struct {
	branch  string
	commits []string
	prs     []string
}

// gatherCompletion reads branch and commit from the project that holds the
// workspace, not from cwd, so archiving from another repo records the right one.
func gatherCompletion(sourceDir, commits, prs string) completionInfo {
	info := completionInfo{}
	if gitAvailable() && gitInsideWorkTree(sourceDir) {
		info.branch, _ = gitOutput(sourceDir, "branch", "--show-current")
		if commits == "" {
			if c, err := gitOutput(sourceDir, "rev-parse", "--short", "HEAD"); err == nil && c != "" {
				info.commits = []string{c}
			}
		}
	}
	if commits != "" {
		info.commits = splitAndTrim(commits, ",")
	}
	if prs != "" {
		info.prs = splitAndTrim(prs, ",")
	}
	return info
}

func (c completionInfo) apply(t *config.Ticket) {
	now := time.Now()
	t.Status = "completed"
	t.CompletedAt = &now
	if len(c.commits) > 0 {
		t.Commits = c.commits
	}
	if len(c.prs) > 0 {
		t.PullRequests = c.prs
	}
	if c.branch != "" {
		if t.Notes == "" {
			t.Notes = fmt.Sprintf("Branch: %s", c.branch)
		} else if !strings.Contains(t.Notes, c.branch) {
			t.Notes = fmt.Sprintf("%s\nBranch: %s", t.Notes, c.branch)
		}
	}
}

func (c completionInfo) changelogEntry() string {
	branch := c.branch
	if branch == "" {
		branch = "-"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\n### %s | %s\n\n**Summary:** Ticket completed and archived\n", time.Now().Format("2006-01-02 15:04"), branch)
	if len(c.commits) > 0 || len(c.prs) > 0 {
		b.WriteString("\n")
	}
	if len(c.commits) > 0 {
		fmt.Fprintf(&b, "- Commits: %s\n", strings.Join(c.commits, ", "))
	}
	if len(c.prs) > 0 {
		fmt.Fprintf(&b, "- PRs: %s\n", strings.Join(c.prs, ", "))
	}
	return b.String()
}

// filterIDCollisions drops files that belong to a longer ticket ID containing
// id: IsTicketFileName treats "-" as a word boundary, so CBP-7 also matches
// CBP-7-x.md.
func filterIDCollisions(files []string, id string, ticketIDs []string) []string {
	kept := []string{}
	for _, f := range files {
		name := filepath.Base(f)
		owner := ""
		for _, other := range ticketIDs {
			if len(other) > len(id) && strings.Contains(strings.ToLower(other), strings.ToLower(id)) && IsTicketFileName(name, other) {
				owner = other
				break
			}
		}
		if owner != "" {
			warningMsg(fmt.Sprintf("skipped %s: belongs to %s", name, owner))
			continue
		}
		kept = append(kept, f)
	}
	return kept
}

var shellSafe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// shellQuote makes a file name safe to paste into a shell.
func shellQuote(s string) string {
	if shellSafe.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// legacyTicketError explains how to clean up a single-file ticket by hand.
func legacyTicketError(cfgMgr *config.Manager, cfg *config.Config, t *config.Ticket) error {
	if t.ArchivedPath != "" {
		return fmt.Errorf("%s is already archived at %s; archive any leftover files by name with `cctx archive <file>...`",
			t.TicketID, filepath.Join(cfgMgr.GetRepoRoot(), t.ArchivedPath))
	}

	cwd, _ := os.Getwd()
	names := []string{}
	for _, f := range filterIDCollisions(FindTicketFiles(cwd, t.TicketID), t.TicketID, cfg.AllTicketIDs()) {
		if info, err := os.Lstat(f); err == nil && info.Mode().IsRegular() {
			names = append(names, shellQuote(filepath.Base(f)))
		}
	}
	if info, err := os.Lstat(filepath.Join(cwd, "SESSIONS.md")); err == nil && info.Mode().IsRegular() {
		names = append(names, "SESSIONS.md")
	}
	if len(names) == 0 {
		return fmt.Errorf("%s is a legacy single-file ticket and none of its files are in %s; run this from the directory holding them", t.TicketID, cwd)
	}
	return fmt.Errorf("%s is a legacy single-file ticket. Archive its files with:\n  cctx archive %s\nthen drop the record with:\n  cctx ticket delete %s",
		t.TicketID, strings.Join(names, " "), shellQuote(t.TicketID))
}

func listWorkspaceFiles(ws string) {
	filepath.WalkDir(ws, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(ws, path)
			fmt.Printf("  - %s\n", rel)
		}
		return nil
	})
}

// archiveTicket archives a directory ticket's workspace, completing it first
// when it is still active.
func archiveTicket(cfgMgr *config.Manager, id string, o ticketArchiveOpts) error {
	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	t := cfg.GetTicket(id, true)
	if t == nil {
		return fmt.Errorf("ticket not found: %s", id)
	}
	if t.WorkspacePath == "" {
		return legacyTicketError(cfgMgr, cfg, t)
	}
	ws := t.WorkspacePath
	if c := canon(ws); filepath.Base(c) != id || filepath.Base(filepath.Dir(c)) != ".cctx" {
		return fmt.Errorf("refusing to archive %s for %s: a ticket workspace must be .cctx/%s", ws, id, id)
	}
	record := config.ArchiveRecord{TicketID: id, Kind: "ticket", SourcePath: ws}

	if t.ArchivedPath != "" && cfg.GetTicket(id, false) == nil {
		if o.commits != "" || o.prs != "" {
			return fmt.Errorf("--commits/--prs are only valid when completing an active ticket")
		}
		archived := filepath.Join(cfgMgr.GetRepoRoot(), t.ArchivedPath)
		if !common.DirExists(ws) {
			infoMsg(fmt.Sprintf("%s is already archived at %s", id, archived))
			return nil
		}
		infoMsg(fmt.Sprintf("%s is already archived at %s, but its workspace still has files:", id, archived))
		listWorkspaceFiles(ws)
		if !dryRun && !o.force && !common.Confirm("Proceed?", true) {
			infoMsg("Operation cancelled")
			return nil
		}
		final, err := archiveItems(cfgMgr, []archiveItem{{Src: ws, RelDest: "."}}, archiveOpts{Name: id, OwnTicketID: id, Record: record})
		if err == nil && final != "" {
			successMsg(fmt.Sprintf("Archived leftovers of %s → %s", id, final))
		}
		return err
	}

	if !common.DirExists(ws) {
		return fmt.Errorf("no files found for %s (expected %s). If the project moved, archive the workspace by path from its new location; otherwise drop the record with `cctx ticket delete %s`", id, ws, id)
	}

	completing := t.Status == "active" || (o.completeAbandoned && t.Status == "abandoned")
	if !completing && (o.commits != "" || o.prs != "") {
		return fmt.Errorf("--commits/--prs are only valid when completing an active ticket")
	}

	fmt.Println()
	infoMsg(fmt.Sprintf("Archiving %s from %s:", id, ws))
	listWorkspaceFiles(ws)
	fmt.Println()
	if !dryRun && !o.force {
		if completing && !o.approveCompletion {
			if !common.Confirm(fmt.Sprintf("%s is active — mark completed and archive?", id), false) {
				infoMsg("Operation cancelled")
				return nil
			}
		} else if !completing && !common.Confirm("Proceed with archive?", false) {
			infoMsg("Operation cancelled")
			return nil
		}
	}

	var completion completionInfo
	opts := archiveOpts{Name: id, OwnTicketID: id, Record: record}
	if completing {
		completion = gatherCompletion(filepath.Dir(filepath.Dir(ws)), o.commits, o.prs)
		opts.Decorate = func(staging string) error {
			changelog := filepath.Join(staging, "changelog.md")
			// A symlinked changelog would send the note outside the archive
			if info, err := os.Lstat(changelog); err != nil || !info.Mode().IsRegular() {
				return nil
			}
			f, err := os.OpenFile(changelog, os.O_APPEND|os.O_WRONLY, 0)
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = f.WriteString(completion.changelogEntry())
			return err
		}
	}
	opts.Mutate = func(cfg *config.Config, archiveRel string) error {
		idx := -1
		for i := range cfg.Tickets.Active {
			if cfg.Tickets.Active[i].TicketID == id {
				idx = i
				break
			}
		}
		if idx < 0 {
			return fmt.Errorf("ticket %s is no longer active in config", id)
		}
		ticket := cfg.Tickets.Active[idx]
		if completing {
			completion.apply(&ticket)
		}
		ticket.ArchivedPath = archiveRel
		ticket.LastModified = time.Now()
		cfg.Tickets.Active = append(cfg.Tickets.Active[:idx], cfg.Tickets.Active[idx+1:]...)
		cfg.Tickets.Archived = append(cfg.Tickets.Archived, ticket)
		cfg.RemoveCurrentWorkingTicket(id)
		return nil
	}

	final, err := archiveItems(cfgMgr, []archiveItem{{Src: ws, RelDest: "."}}, opts)
	if err != nil {
		return err
	}
	if dryRun {
		if completing {
			dryRunMsg(fmt.Sprintf("Would mark %s completed", id))
		}
		return nil
	}
	if completing {
		successMsg(fmt.Sprintf("Completed and archived %s", id))
		if len(completion.commits) > 0 {
			infoMsg(fmt.Sprintf("Commits: %s", strings.Join(completion.commits, ", ")))
		}
	} else {
		successMsg(fmt.Sprintf("Archived %s", id))
	}
	infoMsg(fmt.Sprintf("Location: %s", final))
	return nil
}

// runArchiveAllActive archives every active ticket, optionally for one project.
func runArchiveAllActive(cfgMgr *config.Manager) error {
	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	var projectFilter *config.Project
	if projectFlag != "" || os.Getenv("CCTX_PROJECT") != "" {
		name, err := GetProjectContext(cfgMgr.GetRepoRoot())
		if err != nil {
			return err
		}
		if projectFilter = cfg.GetProject(name); projectFilter == nil {
			return fmt.Errorf("project not found: %s", name)
		}
	}

	selected, legacy := []string{}, []string{}
	for _, t := range cfg.Tickets.Active {
		if t.WorkspacePath == "" {
			if projectFilter == nil {
				legacy = append(legacy, t.TicketID)
				continue
			}
			for _, lp := range t.LinkedProjects {
				if lp.ContextName == projectFilter.ContextName {
					legacy = append(legacy, t.TicketID)
					break
				}
			}
			continue
		}
		if projectFilter != nil && !isWithin(canon(t.WorkspacePath), canon(projectFilter.ProjectPath)) {
			continue
		}
		selected = append(selected, t.TicketID)
	}

	if len(selected) == 0 && len(legacy) == 0 {
		infoMsg("No active tickets to archive")
		return nil
	}
	summary := func(failed int) string {
		return fmt.Sprintf("Archived %d of %d tickets (%d legacy skipped)", len(selected)-failed, len(selected), len(legacy))
	}
	fmt.Println()
	infoMsg(fmt.Sprintf("Archiving %d active ticket(s):", len(selected)))
	for _, id := range selected {
		fmt.Printf("  - %s\n", id)
	}
	for _, id := range legacy {
		fmt.Printf("  - %s (skipped: legacy)\n", id)
	}
	fmt.Println()
	if len(selected) == 0 {
		infoMsg(summary(0))
		return nil
	}
	if !dryRun && !archiveForce && !common.Confirm("Proceed?", false) {
		infoMsg("Operation cancelled")
		return nil
	}

	failed := 0
	for _, id := range selected {
		if err := archiveTicket(cfgMgr, id, ticketArchiveOpts{approveCompletion: true, force: true}); err != nil {
			errorMsg(fmt.Sprintf("%s: %v", id, err))
			failed++
		}
	}
	fmt.Println()
	if failed > 0 {
		return fmt.Errorf("%s", summary(failed))
	}
	successMsg(summary(0))
	return nil
}
