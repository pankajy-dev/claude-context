package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pankaj/claude-context/internal/config"
	"github.com/spf13/cobra"
)

func newWorkspaceEnv(t *testing.T) *TestHelper {
	t.Helper()
	h := newArchiveEnv(t)
	ticketTitle, ticketTags, ticketNotes, ticketForce = "", "", "", false
	t.Cleanup(func() { ticketForce = false })
	if err := os.Chdir(h.projectDir); err != nil {
		t.Fatal(err)
	}
	return h
}

func createTicket(t *testing.T, id string) error {
	t.Helper()
	return runTicketCreate(&cobra.Command{}, []string{id})
}

func TestTicketCreate_UserTemplateOverride(t *testing.T) {
	h := newWorkspaceEnv(t)
	writeFile(t, filepath.Join(h.dataDir, "templates", "ticket-plan.md"), "custom plan for {{TICKET_ID}}")
	if err := createTicket(t, "CBP-2"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(h.projectDir, ".cctx", "CBP-2", "plan.md"))
	if string(got) != "custom plan for CBP-2" {
		t.Errorf("plan.md = %q", got)
	}
}

func TestTicketCreate_Refusals(t *testing.T) {
	h := newWorkspaceEnv(t)

	writeFile(t, filepath.Join(h.projectDir, ".cctx", "CBP-3", "x.md"), "x")
	if err := createTicket(t, "CBP-3"); err == nil || !strings.Contains(err.Error(), "workspace already exists") {
		t.Errorf("existing workspace: %v", err)
	}

	for _, id := range []string{"../x", "CBP-1.md", "a/b"} {
		if err := createTicket(t, id); err == nil || !strings.Contains(err.Error(), "invalid ticket ID") {
			t.Errorf("%s: %v", id, err)
		}
	}

	// <cwd>/.cctx is the data dir itself
	os.Chdir(h.tempDir)
	if err := createTicket(t, "CBP-4"); err == nil || !strings.Contains(err.Error(), "data directory") {
		t.Errorf("data dir: %v", err)
	}

	linked := filepath.Join(h.tempDir, "linked")
	os.MkdirAll(linked, 0755)
	os.Symlink(h.projectDir, filepath.Join(linked, ".cctx"))
	os.Chdir(linked)
	if err := createTicket(t, "CBP-5"); err == nil || !strings.Contains(err.Error(), "must be a real directory") {
		t.Errorf("symlinked .cctx: %v", err)
	}

	cfg := h.GetConfig()
	if len(cfg.Tickets.Active) != 0 {
		t.Errorf("refused creates left tickets in config: %+v", cfg.Tickets.Active)
	}
}

func TestTicketCreate_BranchLastSegment(t *testing.T) {
	h := newWorkspaceEnv(t)
	initGitRepo(t, h.projectDir)
	gitRun(t, h.projectDir, "checkout", "-q", "-b", "feature/CBP-7")
	if err := runTicketCreate(&cobra.Command{}, nil); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(h.projectDir, ".cctx", "CBP-7", "requirements.md")) {
		t.Error("branch feature/CBP-7 should create .cctx/CBP-7/")
	}
}

func TestTicketCreate_Warnings(t *testing.T) {
	h := newWorkspaceEnv(t)
	plain := filepath.Join(h.tempDir, "plain")
	os.MkdirAll(plain, 0755)
	os.Chdir(plain)
	out := captureStdout(t, func() {
		if err := createTicket(t, "CBP-8"); err != nil {
			t.Error(err)
		}
	})
	if !strings.Contains(out, "not a git repository root") || strings.Contains(out, "not git-ignored") {
		t.Errorf("non-git warnings:\n%s", out)
	}

	repo := filepath.Join(h.tempDir, "repo")
	os.MkdirAll(repo, 0755)
	initGitRepo(t, repo)
	os.Chdir(repo)
	h.configMgr.Save(func() *config.Config { c := h.GetConfig(); c.CurrentWorkingTickets = nil; return c }())
	out = captureStdout(t, func() { createTicket(t, "CBP-9") })
	if !strings.Contains(out, "not git-ignored") || strings.Contains(out, "not a git repository root") {
		t.Errorf("unignored repo warnings:\n%s", out)
	}

	writeFile(t, filepath.Join(repo, ".git", "info", "exclude"), ".cctx/\n")
	h.configMgr.Save(func() *config.Config { c := h.GetConfig(); c.CurrentWorkingTickets = nil; return c }())
	out = captureStdout(t, func() { createTicket(t, "CBP-10") })
	if strings.Contains(out, "not git-ignored") {
		t.Errorf("ignored repo still warns:\n%s", out)
	}
}

func TestDirectoryTicket_NonGitEndToEnd(t *testing.T) {
	h := newWorkspaceEnv(t)
	if hasGitAncestor(h.projectDir) {
		t.Skip("temp dir is inside a git repo")
	}
	if err := createTicket(t, "CBP-11"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(h.projectDir, "scratch", "a.md"), "a")
	writeFile(t, filepath.Join(h.projectDir, "loose-1.md"), "b")
	os.Chdir(h.tempDir)
	archiveForce = true
	for _, args := range [][]string{{"CBP-11"}, {filepath.Join(h.projectDir, "scratch")}, {filepath.Join(h.projectDir, "loose-*.md")}} {
		if err := runArchive(archiveCmd, args); err != nil {
			t.Errorf("archive %v in non-git dir: %v", args, err)
		}
	}
}

func archiveDirTicket(t *testing.T, id string) {
	t.Helper()
	archiveForce = true
	defer func() { archiveForce = false }()
	if err := runArchive(archiveCmd, []string{id}); err != nil {
		t.Fatalf("archive %s: %v", id, err)
	}
}

func TestTicketRestore_DirectoryTicketRoundTrip(t *testing.T) {
	h := newWorkspaceEnv(t)
	if err := createTicket(t, "CBP-12"); err != nil {
		t.Fatal(err)
	}
	ws := filepath.Join(h.projectDir, ".cctx", "CBP-12")
	writeFile(t, filepath.Join(ws, "notes", "extra.md"), "extra")
	archiveDirTicket(t, "CBP-12")

	other := filepath.Join(h.tempDir, "other")
	os.MkdirAll(other, 0755)
	os.Chdir(other)
	ticketForce = true
	if err := runTicketRestore(ticketRestoreCmd, []string{"CBP-12"}); err != nil {
		t.Fatalf("restore: %v", err)
	}
	restored := filepath.Join(other, ".cctx", "CBP-12")
	if got, _ := os.ReadFile(filepath.Join(restored, "notes", "extra.md")); string(got) != "extra" {
		t.Errorf("restored content = %q", got)
	}
	cfg := h.GetConfig()
	tk := cfg.GetTicket("CBP-12", false)
	if tk == nil || tk.Status != "active" || tk.WorkspacePath != canon(restored) || tk.ArchivedPath != "" {
		t.Fatalf("ticket after restore: %+v", tk)
	}
	if len(cfg.Archives) != 0 || len(archivedDirs(t, h)) != 0 {
		t.Errorf("archive and record should be gone: %+v %v", cfg.Archives, archivedDirs(t, h))
	}

	// Archive again, then restore into a dir where the target already exists
	os.Chdir(h.tempDir)
	archiveDirTicket(t, "CBP-12")
	writeFile(t, filepath.Join(other, ".cctx", "CBP-12", "squatter.md"), "x")
	os.Chdir(other)
	if err := runTicketRestore(ticketRestoreCmd, []string{"CBP-12"}); err == nil || !strings.Contains(err.Error(), "overwrite") {
		t.Errorf("existing target: %v", err)
	}
}

func TestTicketRestore_FailuresChangeNothing(t *testing.T) {
	h := newWorkspaceEnv(t)
	createTicket(t, "CBP-13")
	os.Chdir(h.tempDir)
	archiveDirTicket(t, "CBP-13")
	target := filepath.Join(h.tempDir, "back")
	os.MkdirAll(target, 0755)
	os.Chdir(target)
	ticketForce = true
	before, _ := os.ReadFile(filepath.Join(h.dataDir, "config.json"))

	copyEntryFn = func(string, string) error { return errors.New("disk full") }
	err := runTicketRestore(ticketRestoreCmd, []string{"CBP-13"})
	copyEntryFn = copyEntry
	if err == nil {
		t.Fatal("expected copy failure")
	}

	saveConfigFn = func(*config.Manager, *config.Config) error { return errors.New("read-only") }
	err = runTicketRestore(ticketRestoreCmd, []string{"CBP-13"})
	saveConfigFn = func(m *config.Manager, c *config.Config) error { return m.Save(c) }
	if err == nil {
		t.Fatal("expected save failure")
	}

	after, _ := os.ReadFile(filepath.Join(h.dataDir, "config.json"))
	if string(before) != string(after) {
		t.Error("config changed")
	}
	if exists(filepath.Join(target, ".cctx", "CBP-13")) || len(archivedDirs(t, h)) != 1 {
		t.Error("failed restore left a target or lost the archive")
	}
	entries, _ := os.ReadDir(filepath.Join(target, ".cctx"))
	if len(entries) != 0 {
		t.Errorf("staging left behind: %v", entries)
	}

	for _, bad := range []string{"", "../..", "contexts/../../escape"} {
		cfg := h.GetConfig()
		for i := range cfg.Tickets.Archived {
			if cfg.Tickets.Archived[i].TicketID == "CBP-13" {
				cfg.Tickets.Archived[i].ArchivedPath = bad
			}
		}
		h.configMgr.Save(cfg)
		if err := runTicketRestore(ticketRestoreCmd, []string{"CBP-13"}); err == nil {
			t.Errorf("archived_path %q: expected error", bad)
		}
	}
	if len(archivedDirs(t, h)) != 1 {
		t.Error("suspicious archived_path touched the archive")
	}
}

func TestDirectoryTicket_LinkDeleteVerify(t *testing.T) {
	h := newWorkspaceEnv(t)
	h.AddProject("test-project", h.projectDir)
	createTicket(t, "CBP-14")
	if err := runTicketLink(ticketLinkCmd, []string{"CBP-14", "test-project"}); err == nil || !strings.Contains(err.Error(), "cannot be linked") {
		t.Errorf("link: %v", err)
	}
	if err := runTicketUnlink(ticketUnlinkCmd, []string{"CBP-14", "test-project"}); err == nil || !strings.Contains(err.Error(), "cannot be linked") {
		t.Errorf("unlink: %v", err)
	}

	// An archive made by glob that merely carries the ID must survive delete
	writeFile(t, filepath.Join(h.projectDir, "CBP-14-notes.md"), "n")
	archiveForce = true
	if err := runArchive(archiveCmd, []string{"CBP-14-notes.md"}); err != nil {
		t.Fatal(err)
	}
	archiveForce = false
	globDir := archivedDirs(t, h)[0]

	ticketForce = true
	os.Chdir(h.tempDir)
	if err := runTicketDelete(ticketDeleteCmd, []string{"CBP-14"}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	cfg := h.GetConfig()
	if cfg.GetTicket("CBP-14", true) != nil || exists(filepath.Join(h.projectDir, ".cctx", "CBP-14")) {
		t.Error("ticket or workspace not deleted")
	}
	if len(cfg.Archives) != 1 || cfg.Archives[0].TicketID != "" || !exists(filepath.Join(h.dataDir, "contexts", "_archived", globDir)) {
		t.Errorf("glob archive should stay with TicketID cleared: %+v", cfg.Archives)
	}

	// workspace_path hand-edited to the project root: nothing on disk is deleted
	os.Chdir(h.projectDir)
	h.configMgr.Save(func() *config.Config { c := h.GetConfig(); c.CurrentWorkingTickets = nil; return c }())
	createTicket(t, "CBP-15")
	cfg = h.GetConfig()
	for i := range cfg.Tickets.Active {
		if cfg.Tickets.Active[i].TicketID == "CBP-15" {
			cfg.Tickets.Active[i].WorkspacePath = h.projectDir
		}
	}
	h.configMgr.Save(cfg)
	os.Chdir(h.tempDir)
	if err := runTicketDelete(ticketDeleteCmd, []string{"CBP-15"}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if !exists(filepath.Join(h.projectDir, ".cctx", "CBP-15", "plan.md")) {
		t.Error("hand-edited workspace_path caused a deletion")
	}
	if h.GetConfig().GetTicket("CBP-15", true) != nil {
		t.Error("config entry should still be removed")
	}
}

func TestVerify_ReportsMissingWorkspace(t *testing.T) {
	h := newWorkspaceEnv(t)
	createTicket(t, "CBP-16")
	os.RemoveAll(filepath.Join(h.projectDir, ".cctx", "CBP-16"))
	out := captureStdout(t, func() { runVerify(verifyCmd, nil) })
	if !strings.Contains(out, "missing-workspace") {
		t.Errorf("verify output:\n%s", out)
	}
	if strings.Contains(out, "CBP-16.md") {
		t.Errorf("verify should not check symlinks for directory tickets:\n%s", out)
	}
}

func TestInitSpecs(t *testing.T) {
	h := newWorkspaceEnv(t)
	if err := runInitSpecs(initSpecsCmd, nil); err != nil {
		t.Fatal(err)
	}
	mission := filepath.Join(h.projectDir, ".cctx", "mission.md")
	for _, f := range []string{"mission.md", "tech-stack.md", "roadmap.md"} {
		if !exists(filepath.Join(h.projectDir, ".cctx", f)) {
			t.Errorf("missing %s", f)
		}
	}
	writeFile(t, mission, "edited")
	if err := runInitSpecs(initSpecsCmd, nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(mission); string(got) != "edited" {
		t.Error("init-specs overwrote an existing file")
	}

	os.Chdir(h.tempDir)
	if err := runInitSpecs(initSpecsCmd, nil); err == nil {
		t.Error("init-specs inside the data dir should fail")
	}
}

func TestLegacyDelete_SuspiciousArchivedPathNotRemoved(t *testing.T) {
	h := newWorkspaceEnv(t)
	victim := filepath.Join(h.tempDir, "victim")
	writeFile(t, filepath.Join(victim, "keep.md"), "precious")
	cfg := h.GetConfig()
	cfg.Tickets.Archived = append(cfg.Tickets.Archived, config.Ticket{TicketID: "OLD-5", Status: "completed", ArchivedPath: "../victim"})
	h.configMgr.Save(cfg)

	ticketForce = true
	os.Chdir(h.tempDir)
	if err := runTicketDelete(ticketDeleteCmd, []string{"OLD-5"}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if !exists(filepath.Join(victim, "keep.md")) {
		t.Fatal("legacy delete removed a directory outside contexts/_archived")
	}
}

func TestLegacyCleanup_HowToThenDeleteKeepsSessions(t *testing.T) {
	h := newWorkspaceEnv(t)
	other := filepath.Join(h.tempDir, "other")
	writeFile(t, filepath.Join(other, "SESSIONS.md"), "another ticket's log")
	cfg := h.GetConfig()
	cfg.ManagedProjects = append(cfg.ManagedProjects, config.Project{ContextName: "other", ProjectPath: other})
	cfg.Tickets.Active = append(cfg.Tickets.Active, config.Ticket{TicketID: "OLD-6", Status: "active"})
	h.configMgr.Save(cfg)
	for _, f := range []string{"OLD-6.md", "PLAN-OLD-6.md", "SESSIONS.md"} {
		writeFile(t, filepath.Join(h.projectDir, f), f)
	}

	archiveForce = true
	if err := runArchive(archiveCmd, []string{"OLD-6.md", "PLAN-OLD-6.md", "SESSIONS.md"}); err != nil {
		t.Fatal(err)
	}
	archiveForce = false
	ticketForce = true
	if err := runTicketDelete(ticketDeleteCmd, []string{"OLD-6"}); err != nil {
		t.Fatal(err)
	}

	cfg = h.GetConfig()
	if len(cfg.Archives) != 1 || cfg.Archives[0].FileCount != 3 {
		t.Fatalf("expected the how-to archive to survive delete: %+v", cfg.Archives)
	}
	if !exists(filepath.Join(h.dataDir, cfg.Archives[0].ArchiveDir, "SESSIONS.md")) {
		t.Error("SESSIONS.md should be in the archive")
	}
	if got, _ := os.ReadFile(filepath.Join(other, "SESSIONS.md")); string(got) != "another ticket's log" {
		t.Error("legacy delete removed another project's SESSIONS.md")
	}
}

func TestLegacyRestore_RemovesArchiveRecord(t *testing.T) {
	h := newWorkspaceEnv(t)
	rel := filepath.Join("contexts", "_archived", "2026-01-01_OLD-7")
	writeFile(t, filepath.Join(h.dataDir, rel, "OLD-7.md"), "t")
	cfg := h.GetConfig()
	cfg.Tickets.Archived = append(cfg.Tickets.Archived, config.Ticket{TicketID: "OLD-7", Status: "completed", ArchivedPath: rel})
	cfg.Archives = append(cfg.Archives, config.ArchiveRecord{ArchiveDir: rel, TicketID: "OLD-7", Kind: "backfill", FileCount: 1})
	h.configMgr.Save(cfg)

	ticketForce = true
	if err := runTicketRestore(ticketRestoreCmd, []string{"OLD-7"}); err != nil {
		t.Fatal(err)
	}
	if n := len(h.GetConfig().Archives); n != 0 {
		t.Errorf("record should be removed once the archive is emptied, have %d", n)
	}
}

func TestDirectoryTicketDelete_AbortsFromInsideAndRemovesAllTicketArchives(t *testing.T) {
	h := newWorkspaceEnv(t)
	createTicket(t, "CBP-61")
	ws := filepath.Join(h.projectDir, ".cctx", "CBP-61")
	ticketForce = true

	os.Chdir(ws)
	if err := runTicketDelete(ticketDeleteCmd, []string{"CBP-61"}); !errors.Is(err, errCwdInside) {
		t.Fatalf("delete from inside the workspace: %v", err)
	}
	if h.GetConfig().GetTicket("CBP-61", false) == nil || !exists(ws) {
		t.Fatal("aborted delete changed state")
	}

	os.Chdir(h.tempDir)
	archiveDirTicket(t, "CBP-61")
	writeFile(t, filepath.Join(ws, "late.md"), "x")
	archiveDirTicket(t, "CBP-61")
	if n := len(archivedDirs(t, h)); n != 2 {
		t.Fatalf("setup: %d archive dirs", n)
	}
	if err := runTicketDelete(ticketDeleteCmd, []string{"CBP-61"}); err != nil {
		t.Fatal(err)
	}
	if n := len(archivedDirs(t, h)); n != 0 || len(h.GetConfig().Archives) != 0 {
		t.Errorf("ticket archives left: %v %+v", archivedDirs(t, h), h.GetConfig().Archives)
	}
}

func TestTicketShow_ResolvesFromBranch(t *testing.T) {
	h := newWorkspaceEnv(t)
	initGitRepo(t, h.projectDir)
	gitRun(t, h.projectDir, "checkout", "-q", "-b", "feature/CBP-62")
	createTicket(t, "CBP-62")
	out := captureStdout(t, func() {
		if err := runTicketShow(ticketShowCmd, nil); err != nil {
			t.Error(err)
		}
	})
	if !strings.Contains(out, "Workspace: "+canon(filepath.Join(h.projectDir, ".cctx", "CBP-62"))) {
		t.Errorf("show output:\n%s", out)
	}
}

func TestTicketDelete_UnknownIDCannotEscapeTicketsDir(t *testing.T) {
	h := newWorkspaceEnv(t)
	keep := filepath.Join(h.dataDir, "contexts", "_archived", "2026-01-01_KEEP")
	writeFile(t, filepath.Join(keep, "a.md"), "a")
	ticketForce = true
	os.Chdir(h.tempDir)
	runTicketDelete(ticketDeleteCmd, []string{"../_archived"})
	if !exists(filepath.Join(keep, "a.md")) {
		t.Fatal("ticket delete ../_archived wiped the archives")
	}
}
