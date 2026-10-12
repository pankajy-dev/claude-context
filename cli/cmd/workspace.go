package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pankaj/claude-context/internal/common"
	"github.com/pankaj/claude-context/internal/config"
	"github.com/pankaj/claude-context/internal/templates"
)

// ticketTemplateFiles maps the files scaffolded into .cctx/<ID>/ to their templates.
var ticketTemplateFiles = []struct{ file, template string }{
	{"requirements.md", "ticket-requirements"},
	{"plan.md", "ticket-plan"},
	{"validation.md", "ticket-validation"},
	{"changelog.md", "ticket-changelog"},
}

var specsTemplateFiles = []struct{ file, template string }{
	{"mission.md", "specs-mission"},
	{"tech-stack.md", "specs-tech-stack"},
	{"roadmap.md", "specs-roadmap"},
}

// ensureWorkspaceDir runs the checks shared by everything that writes a
// project workspace (.cctx/ in cwd) and returns its path. Nothing is created.
func ensureWorkspaceDir(cwd, dataDir string) (string, error) {
	ws := filepath.Join(cwd, ".cctx")

	for _, root := range []string{dataDir, defaultDataDir()} {
		if root != "" && isWithin(canon(ws), canon(root)) {
			return "", fmt.Errorf("refusing to create a workspace inside the cctx data directory (%s)", root)
		}
	}
	if info, err := os.Lstat(ws); err == nil && (info.Mode()&os.ModeSymlink != 0 || !info.IsDir()) {
		return "", fmt.Errorf("%s must be a real directory", ws)
	}

	if _, err := os.Lstat(filepath.Join(cwd, ".git")); err != nil {
		warningMsg(fmt.Sprintf("Current directory is not a git repository root; .cctx/ will be created in %s", cwd))
	}
	if gitAvailable() && gitInsideWorkTree(cwd) {
		cmd := gitCommand("-c", "core.fsmonitor=false", "-C", cwd, "check-ignore", "-q", ".cctx/")
		var exitErr *exec.ExitError
		if err := cmd.Run(); errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			warningMsg(".cctx/ is not git-ignored. Add it globally: echo '.cctx/' >> ~/.gitignore_global")
		}
	}
	return ws, nil
}

// writeTemplateFile renders a template into path, refusing to overwrite.
func writeTemplateFile(path, template, ticketID, dataDir string) error {
	content, _, err := templates.GetTemplate(template, dataDir)
	if err != nil {
		return fmt.Errorf("failed to load template %s: %w", template, err)
	}
	rendered := strings.ReplaceAll(string(content), "{{TICKET_ID}}", ticketID)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(rendered); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// scaffoldTicketWorkspace creates .cctx/<ID>/ with the ticket templates. On
// failure the directory is removed so a retry isn't blocked.
func scaffoldTicketWorkspace(ws, ticketID, dataDir string) (string, error) {
	dir := filepath.Join(ws, ticketID)
	if _, err := os.Lstat(dir); err == nil {
		return "", fmt.Errorf("workspace already exists: %s", dir)
	}
	if err := os.MkdirAll(ws, 0755); err != nil {
		return "", fmt.Errorf("failed to create %s: %w", ws, err)
	}
	if err := os.Mkdir(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create %s: %w", dir, err)
	}
	for _, f := range ticketTemplateFiles {
		if err := writeTemplateFile(filepath.Join(dir, f.file), f.template, ticketID, dataDir); err != nil {
			os.RemoveAll(dir)
			return "", fmt.Errorf("failed to write %s: %w", f.file, err)
		}
	}
	return dir, nil
}

// safeArchiveDir resolves an archived_path and reports whether it sits
// directly under contexts/_archived/ — anything else is never touched.
func safeArchiveDir(cfgMgr *config.Manager, rel string) (string, bool) {
	if rel == "" {
		return "", false
	}
	abs := filepath.Join(cfgMgr.GetRepoRoot(), rel)
	root := canon(filepath.Join(cfgMgr.GetContextsPath(), "_archived"))
	return abs, filepath.Dir(canon(abs)) == root
}

// restoreDirectoryTicket copies an archived workspace back to ./.cctx/<ID>/.
func restoreDirectoryTicket(cfgMgr *config.Manager, ticket *config.Ticket) error {
	id := ticket.TicketID
	if !validTicketID(id) {
		return fmt.Errorf("invalid ticket ID %q", id)
	}
	archive, ok := safeArchiveDir(cfgMgr, ticket.ArchivedPath)
	if !ok {
		return fmt.Errorf("refusing suspicious archived_path %q", ticket.ArchivedPath)
	}
	if !common.DirExists(archive) {
		return fmt.Errorf("archived directory not found: %s", archive)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	wsRoot, err := ensureWorkspaceDir(cwd, cfgMgr.GetRepoRoot())
	if err != nil {
		return err
	}
	target := filepath.Join(wsRoot, id)
	if _, err := os.Lstat(target); err == nil {
		return fmt.Errorf("refusing to overwrite existing workspace: %s", target)
	}

	cfg, err := cfgMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	for _, r := range cfg.Archives {
		if r.TicketID == id && r.Kind == "ticket" && filepath.Clean(r.ArchiveDir) != filepath.Clean(ticket.ArchivedPath) {
			warningMsg(fmt.Sprintf("%s also has a later archive with leftover files, not restored: %s", id, filepath.Join(cfgMgr.GetRepoRoot(), r.ArchiveDir)))
		}
	}

	fmt.Println()
	infoMsg(fmt.Sprintf("Restoring ticket: %s", id))
	infoMsg(fmt.Sprintf("From: %s", archive))
	infoMsg(fmt.Sprintf("To: %s", target))
	fmt.Println()
	listWorkspaceFiles(archive)
	fmt.Println()
	if dryRun {
		dryRunMsg(fmt.Sprintf("Would restore %s to %s", id, target))
		return nil
	}
	if !ticketForce && !common.Confirm("Proceed with restore?", true) {
		infoMsg("Operation cancelled")
		return nil
	}

	if err := os.MkdirAll(wsRoot, 0755); err != nil {
		return fmt.Errorf("failed to create %s: %w", wsRoot, err)
	}
	staging, err := os.MkdirTemp(wsRoot, ".staging-")
	if err != nil {
		return fmt.Errorf("failed to create staging directory: %w", err)
	}
	defer os.RemoveAll(staging)
	copied, _, err := copyTree(archive, staging, ".")
	if err != nil {
		return err
	}
	if err := verifyCopies(copied); err != nil {
		return err
	}
	if _, err := os.Lstat(target); err == nil {
		return fmt.Errorf("refusing to overwrite existing workspace: %s", target)
	}
	if err := os.Rename(staging, target); err != nil {
		return fmt.Errorf("failed to move restored workspace into place: %w", err)
	}
	// MkdirTemp creates 0700; give the workspace the archived dir's mode back
	if info, err := os.Stat(archive); err == nil {
		os.Chmod(target, info.Mode().Perm())
	}

	if cfg, err = cfgMgr.Load(); err != nil {
		os.RemoveAll(target)
		return fmt.Errorf("failed to load config: %w", err)
	}
	idx := -1
	for i := range cfg.Tickets.Archived {
		if cfg.Tickets.Archived[i].TicketID == id {
			idx = i
		}
	}
	if idx < 0 {
		os.RemoveAll(target)
		return fmt.Errorf("ticket %s is no longer archived in config", id)
	}
	restored := cfg.Tickets.Archived[idx]
	archivedRel := restored.ArchivedPath
	restored.Status = "active"
	restored.ArchivedPath = ""
	restored.CompletedAt = nil
	restored.AbandonedAt = nil
	restored.WorkspacePath = canon(target)
	restored.LastModified = time.Now()
	cfg.Tickets.Archived = append(cfg.Tickets.Archived[:idx], cfg.Tickets.Archived[idx+1:]...)
	cfg.Tickets.Active = append(cfg.Tickets.Active, restored)
	projectName, _ := GetProjectContext(cfgMgr.GetRepoRoot())
	if projectName == "" {
		projectName = filepath.Base(cwd)
	}
	cfg.AddCurrentWorkingTicket(id, projectName)
	var kept []config.ArchiveRecord
	var record *config.ArchiveRecord
	for _, r := range cfg.Archives {
		if filepath.Clean(r.ArchiveDir) == filepath.Clean(archivedRel) {
			r := r
			record = &r
			continue
		}
		kept = append(kept, r)
	}
	cfg.Archives = kept
	if err := saveConfigFn(cfgMgr, cfg); err != nil {
		os.RemoveAll(target)
		return fmt.Errorf("failed to save config, nothing was restored: %w", err)
	}

	if err := os.RemoveAll(archive); err != nil {
		warningMsg(fmt.Sprintf("Restored, but could not remove the archive %s: %v", archive, err))
		if record == nil {
			record = &config.ArchiveRecord{ArchiveDir: archivedRel, Kind: "ticket", ArchivedAt: time.Now()}
		}
		record.TicketID = ""
		record.FileCount = countArchiveEntries(archive)
		if cfg, err := cfgMgr.Load(); err == nil {
			cfg.Archives = append(cfg.Archives, *record)
			cfgMgr.Save(cfg)
		}
	}

	successMsg(fmt.Sprintf("Successfully restored ticket: %s", id))
	infoMsg(fmt.Sprintf("Workspace: %s", target))
	return nil
}

// deleteDirectoryTicket removes a directory ticket, its workspace and the
// archives that ticket mode created for it.
func deleteDirectoryTicket(cfgMgr *config.Manager, cfg *config.Config, ticket *config.Ticket) error {
	id := ticket.TicketID
	ws := ticket.WorkspacePath

	removeWorkspace := false
	if _, err := os.Lstat(ws); err == nil {
		c := canon(ws)
		switch {
		case filepath.Base(c) != id || filepath.Base(filepath.Dir(c)) != ".cctx":
			warningMsg(fmt.Sprintf("Not removing %s: it does not look like .cctx/%s", ws, id))
		default:
			err := archiveRefusals(cfg, cfgMgr.GetRepoRoot(), []archiveItem{{Src: ws, RelDest: "."}}, archiveOpts{OwnTicketID: id})
			if err != nil {
				if errors.Is(err, errCwdInside) || errors.Is(err, errGitMissing) {
					return err
				}
				warningMsg(fmt.Sprintf("Not removing %s: %v", ws, err))
			} else {
				removeWorkspace = true
			}
		}
	}

	archives := []string{}
	seen := map[string]bool{}
	addArchive := func(rel string) {
		rel = filepath.Clean(rel)
		if rel == "." || seen[rel] {
			return
		}
		seen[rel] = true
		if abs, ok := safeArchiveDir(cfgMgr, rel); ok {
			archives = append(archives, abs)
		} else {
			warningMsg(fmt.Sprintf("Not removing suspicious archive path %q", rel))
		}
	}
	if ticket.ArchivedPath != "" {
		addArchive(ticket.ArchivedPath)
	}
	for _, r := range cfg.Archives {
		if r.TicketID == id && r.Kind == "ticket" {
			addArchive(r.ArchiveDir)
		}
	}

	fmt.Println()
	warningMsg(fmt.Sprintf("About to permanently delete ticket: %s", id))
	warningMsg("This will remove:")
	warningMsg("  - Ticket from config.json")
	if removeWorkspace {
		warningMsg(fmt.Sprintf("  - Workspace %s", ws))
	}
	for _, a := range archives {
		warningMsg(fmt.Sprintf("  - Archive %s", a))
	}
	fmt.Println()
	if dryRun {
		dryRunMsg(fmt.Sprintf("Would delete ticket %s", id))
		return nil
	}
	if !ticketForce && !common.Confirm("Are you sure you want to delete this ticket?", false) {
		infoMsg("Operation cancelled")
		return nil
	}

	// Save first: a failed save must not leave config pointing at removed dirs
	planned := map[string]bool{}
	for _, a := range archives {
		planned[filepath.Join("contexts", "_archived", filepath.Base(a))] = true
	}
	var active, archived []config.Ticket
	for _, t := range cfg.Tickets.Active {
		if t.TicketID != id {
			active = append(active, t)
		}
	}
	for _, t := range cfg.Tickets.Archived {
		if t.TicketID != id {
			archived = append(archived, t)
		}
	}
	cfg.Tickets.Active, cfg.Tickets.Archived = active, archived
	var records []config.ArchiveRecord
	for _, r := range cfg.Archives {
		if planned[filepath.Clean(r.ArchiveDir)] {
			continue
		}
		if r.TicketID == id {
			r.TicketID = ""
		}
		records = append(records, r)
	}
	cfg.Archives = records
	cfg.RemoveCurrentWorkingTicket(id)
	if err := cfgMgr.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config, nothing was deleted: %w", err)
	}

	if removeWorkspace {
		if err := os.RemoveAll(ws); err != nil {
			warningMsg(fmt.Sprintf("Failed to remove %s: %v", ws, err))
		}
	}
	for _, a := range archives {
		if err := os.RemoveAll(a); err != nil {
			warningMsg(fmt.Sprintf("Failed to remove %s: %v", a, err))
		}
	}
	successMsg(fmt.Sprintf("Successfully deleted ticket: %s", id))
	return nil
}
