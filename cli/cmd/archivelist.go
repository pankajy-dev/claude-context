package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/pankaj/claude-context/internal/common"
	"github.com/pankaj/claude-context/internal/config"
)

var (
	archiveDatePrefix = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})_(.+)$`)
	archiveNumSuffix  = regexp.MustCompile(`^(.+)-\d+$`)
)

// projectPathForTicket returns the project a legacy ticket's files came from.
func projectPathForTicket(cfg *config.Config, t *config.Ticket) string {
	if t.PrimaryContextName != "" {
		if p := cfg.GetProject(t.PrimaryContextName); p != nil {
			return p.ProjectPath
		}
	}
	if len(t.LinkedProjects) > 0 {
		return t.LinkedProjects[0].ProjectPath
	}
	return ""
}

// backfillArchiveRecords adds a record for every archive dir that predates
// records, guessing the ticket and source from config and the dir name.
func backfillArchiveRecords(cfgMgr *config.Manager, cfg *config.Config) int {
	root := filepath.Join(cfgMgr.GetContextsPath(), "_archived")
	entries, err := os.ReadDir(root)
	if err != nil {
		return 0
	}
	known := map[string]bool{}
	for _, r := range cfg.Archives {
		known[filepath.Clean(r.ArchiveDir)] = true
	}

	added := 0
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		rel := filepath.Join("contexts", "_archived", e.Name())
		if known[rel] {
			continue
		}
		rec := config.ArchiveRecord{ArchiveDir: rel, Kind: "backfill", FileCount: countArchiveEntries(filepath.Join(root, e.Name()))}

		var ticket *config.Ticket
		for _, group := range [][]config.Ticket{cfg.Tickets.Archived, cfg.Tickets.Active} {
			for i := range group {
				if group[i].ArchivedPath != "" && filepath.Clean(group[i].ArchivedPath) == rel {
					ticket = &group[i]
				}
			}
		}

		rest := e.Name()
		if m := archiveDatePrefix.FindStringSubmatch(e.Name()); m != nil {
			rest = m[2]
			if d, err := time.ParseInLocation("2006-01-02", m[1], time.Local); err == nil {
				rec.ArchivedAt = d
			}
		}
		if rec.ArchivedAt.IsZero() {
			if info, err := e.Info(); err == nil {
				rec.ArchivedAt = info.ModTime()
			}
		}
		base := rest
		if m := archiveNumSuffix.FindStringSubmatch(rest); m != nil {
			base = m[1]
		}
		if ticket == nil {
			ticket = cfg.GetTicket(rest, true)
		}
		if ticket == nil && base != rest {
			ticket = cfg.GetTicket(base, true)
		}

		if ticket != nil {
			rec.TicketID = ticket.TicketID
			rec.SourcePath = projectPathForTicket(cfg, ticket)
		}
		if rec.SourcePath == "" {
			for _, p := range cfg.ManagedProjects {
				if p.ContextName == base || filepath.Base(p.ProjectPath) == base {
					rec.SourcePath = p.ProjectPath
					break
				}
			}
		}
		if rec.SourcePath != "" {
			rec.SourcePath = canon(rec.SourcePath)
		}
		cfg.Archives = append(cfg.Archives, rec)
		added++
	}
	return added
}

func runArchiveList(cfgMgr *config.Manager) error {
	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if !dryRun && backfillArchiveRecords(cfgMgr, cfg) > 0 {
		if err := cfgMgr.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
	}

	cwd, _ := os.Getwd()
	scope := canon(cwd)
	if gitAvailable() {
		if top := gitTopLevel(cwd); top != "" {
			scope = top
		}
	}

	records := []config.ArchiveRecord{}
	for _, r := range cfg.Archives {
		if archiveListAll || (r.SourcePath != "" && isWithin(canon(r.SourcePath), scope)) {
			records = append(records, r)
		}
	}
	sort.SliceStable(records, func(i, j int) bool { return records[i].ArchivedAt.After(records[j].ArchivedAt) })
	if archiveLimit > 0 && len(records) > archiveLimit {
		records = records[:archiveLimit]
	}

	if len(records) == 0 {
		infoMsg(fmt.Sprintf("No archives for %s", scope))
		if !archiveListAll {
			infoMsg("Use --all to list archives from everywhere")
		}
		return nil
	}

	fmt.Printf("%-10s  %-40s  %-14s  %5s  %s\n", "DATE", "ARCHIVE", "TICKET", "FILES", "SOURCE")
	for _, r := range records {
		name := filepath.Base(r.ArchiveDir)
		if !common.DirExists(filepath.Join(cfgMgr.GetRepoRoot(), r.ArchiveDir)) {
			name += " (missing)"
		}
		ticket := r.TicketID
		if ticket == "" {
			ticket = "-"
		}
		source := r.SourcePath
		if source == "" {
			source = "unknown"
		}
		fmt.Printf("%-10s  %-40s  %-14s  %5d  %s\n", r.ArchivedAt.Format("2006-01-02"), name, ticket, r.FileCount, source)
	}
	return nil
}
