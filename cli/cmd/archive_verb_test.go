package cmd

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pankaj/claude-context/internal/config"
	"github.com/spf13/pflag"
)

func resetArchiveFlags() {
	archiveForce, archiveAllActive, archiveList, archiveListAll = false, false, false, false
	archiveLimit, archiveCommits, archivePRs = 20, "", ""
	ticketCommits, ticketPRs, ticketFlag, projectFlag = "", "", "", ""
	dryRun = false
	// rootCmd.Execute in one test must not leave flags marked as set for the next
	archiveCmd.Flags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
}

func newArchiveEnv(t *testing.T) *TestHelper {
	t.Helper()
	h := newEngineEnv(t)
	resetArchiveFlags()
	t.Cleanup(resetArchiveFlags)
	return h
}

// addDirTicket creates a directory ticket with a workspace under project/.cctx/<id>.
func addDirTicket(t *testing.T, h *TestHelper, project, id, status string) string {
	t.Helper()
	ws := filepath.Join(project, ".cctx", id)
	writeFile(t, filepath.Join(ws, "requirements.md"), "req")
	writeFile(t, filepath.Join(ws, "changelog.md"), "# Changelog\n")
	cfg := h.GetConfig()
	cfg.Tickets.Active = append(cfg.Tickets.Active, config.Ticket{
		TicketID: id, Status: status, CreatedAt: time.Now(), LastModified: time.Now(),
		WorkspacePath: canon(ws),
	})
	cfg.AddCurrentWorkingTicket(id, "")
	if err := h.configMgr.Save(cfg); err != nil {
		t.Fatal(err)
	}
	return ws
}

func archivedTicket(t *testing.T, h *TestHelper, id string) *config.Ticket {
	t.Helper()
	cfg := h.GetConfig()
	for i := range cfg.Tickets.Archived {
		if cfg.Tickets.Archived[i].TicketID == id {
			return &cfg.Tickets.Archived[i]
		}
	}
	return nil
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
}

func TestArchiveTicket_ActiveTicketCompletedAndArchived(t *testing.T) {
	h := newArchiveEnv(t)
	ws := addDirTicket(t, h, h.projectDir, "CBP-1", "active")
	rootPlan := filepath.Join(h.projectDir, "PLAN-CBP-1.md")
	writeFile(t, rootPlan, "plan")

	archiveForce = true
	if err := runArchive(archiveCmd, []string{"CBP-1"}); err != nil {
		t.Fatalf("runArchive: %v", err)
	}

	tk := archivedTicket(t, h, "CBP-1")
	if tk == nil || tk.Status != "completed" || tk.CompletedAt == nil {
		t.Fatalf("ticket not completed+archived: %+v", tk)
	}
	if tk.WorkspacePath != canon(ws) {
		t.Errorf("workspace_path not retained: %s", tk.WorkspacePath)
	}
	archive := filepath.Join(h.dataDir, tk.ArchivedPath)
	changelog, _ := os.ReadFile(filepath.Join(archive, "changelog.md"))
	if !strings.Contains(string(changelog), "Ticket completed and archived") {
		t.Errorf("changelog missing completion entry: %q", changelog)
	}
	if exists(ws) {
		t.Error("workspace should be gone")
	}
	if !exists(rootPlan) {
		t.Error("files outside the workspace must not be touched")
	}
	cfg := h.GetConfig()
	if cfg.IsCurrentlyWorking("CBP-1") {
		t.Error("still in working tickets")
	}
	if len(cfg.Archives) != 1 || cfg.Archives[0].Kind != "ticket" || cfg.Archives[0].TicketID != "CBP-1" || cfg.Archives[0].ArchiveDir != tk.ArchivedPath {
		t.Errorf("unexpected records: %+v", cfg.Archives)
	}
}

func TestArchiveTicket_DeclinedPromptChangesNothing(t *testing.T) {
	h := newArchiveEnv(t)
	ws := addDirTicket(t, h, h.projectDir, "CBP-2", "active")
	before, _ := os.ReadFile(filepath.Join(h.dataDir, "config.json"))

	// stdin is empty under go test, so the [y/N] prompt takes its default: no
	if err := runArchive(archiveCmd, []string{"CBP-2"}); err != nil {
		t.Fatalf("runArchive: %v", err)
	}
	after, _ := os.ReadFile(filepath.Join(h.dataDir, "config.json"))
	if string(before) != string(after) || !exists(ws) {
		t.Error("declining changed state")
	}
}

func TestArchiveTicket_CompletedButActiveNotRecompleted(t *testing.T) {
	h := newArchiveEnv(t)
	addDirTicket(t, h, h.projectDir, "CBP-3", "completed")

	archiveForce = true
	if err := runArchive(archiveCmd, []string{"CBP-3"}); err != nil {
		t.Fatalf("runArchive: %v", err)
	}
	tk := archivedTicket(t, h, "CBP-3")
	if tk == nil {
		t.Fatal("not archived")
	}
	changelog, _ := os.ReadFile(filepath.Join(h.dataDir, tk.ArchivedPath, "changelog.md"))
	if strings.Contains(string(changelog), "Ticket completed") {
		t.Error("completion must not be re-applied")
	}

	archiveCommits = "abc"
	addDirTicket(t, h, h.projectDir, "CBP-4", "abandoned")
	if err := runArchive(archiveCmd, []string{"CBP-4"}); err == nil {
		t.Error("--commits on an abandoned ticket should error")
	}
}

func TestArchiveTicket_LegacyTickets(t *testing.T) {
	h := newArchiveEnv(t)
	cfg := h.GetConfig()
	cfg.Tickets.Archived = append(cfg.Tickets.Archived, config.Ticket{TicketID: "OLD-1", Status: "completed", ArchivedPath: "contexts/_archived/2026-01-01_OLD-1"})
	cfg.Tickets.Active = append(cfg.Tickets.Active, config.Ticket{TicketID: "OLD-2", Status: "active"})
	h.configMgr.Save(cfg)

	err := runArchive(archiveCmd, []string{"OLD-1"})
	if err == nil || !strings.Contains(err.Error(), "already archived") || strings.Contains(err.Error(), "ticket delete") {
		t.Errorf("archived legacy: %v", err)
	}

	os.Chdir(h.projectDir)
	for _, f := range []string{"OLD-2.md", "PLAN-OLD-2.md", "SESSIONS.md"} {
		writeFile(t, filepath.Join(h.projectDir, f), f)
	}
	err = runArchive(archiveCmd, []string{"OLD-2"})
	if err == nil || !strings.Contains(err.Error(), "SESSIONS.md") || !strings.Contains(err.Error(), "cctx ticket delete OLD-2") {
		t.Fatalf("active legacy: %v", err)
	}

	// Following the printed command puts all three files in one archive dir
	archiveForce = true
	if err := runArchive(archiveCmd, []string{"OLD-2.md", "PLAN-OLD-2.md", "SESSIONS.md"}); err != nil {
		t.Fatalf("archive by name: %v", err)
	}
	cfg = h.GetConfig()
	if len(cfg.Archives) != 1 || cfg.Archives[0].TicketID != "OLD-2" || cfg.Archives[0].FileCount != 3 {
		t.Errorf("expected one archive with 3 files for OLD-2, got %+v", cfg.Archives)
	}
}

func TestArchive_BareIDFallbackSkipsLongerTicketIDs(t *testing.T) {
	h := newArchiveEnv(t)
	cfg := h.GetConfig()
	cfg.Tickets.Archived = append(cfg.Tickets.Archived, config.Ticket{TicketID: "CBP-8-x", Status: "completed"})
	h.configMgr.Save(cfg)
	os.Chdir(h.projectDir)
	writeFile(t, filepath.Join(h.projectDir, "CBP-8.md"), "mine")
	writeFile(t, filepath.Join(h.projectDir, "CBP-8-x.md"), "theirs")

	archiveForce = true
	if err := runArchive(archiveCmd, []string{"CBP-8"}); err != nil {
		t.Fatalf("runArchive: %v", err)
	}
	if exists(filepath.Join(h.projectDir, "CBP-8.md")) {
		t.Error("CBP-8.md should be archived")
	}
	if !exists(filepath.Join(h.projectDir, "CBP-8-x.md")) {
		t.Error("CBP-8-x.md belongs to another ticket")
	}
}

func TestArchive_PathAndGlobModes(t *testing.T) {
	h := newArchiveEnv(t)
	ws := addDirTicket(t, h, h.projectDir, "CBP-5", "active")
	specs := filepath.Join(h.projectDir, "specs", "2026-10-12-login")
	writeFile(t, filepath.Join(specs, "plan.md"), "p")
	os.Chdir(h.projectDir)
	archiveForce = true

	if err := runArchive(archiveCmd, []string{"specs/2026-10-12-login"}); err != nil {
		t.Fatalf("path archive: %v", err)
	}
	cfg := h.GetConfig()
	rec := cfg.Archives[len(cfg.Archives)-1]
	if rec.Kind != "path" || rec.SourcePath != canon(specs) || !strings.HasSuffix(rec.ArchiveDir, "_2026-10-12-login") {
		t.Errorf("unexpected record %+v", rec)
	}

	for _, arg := range []string{".cctx/CBP-5", ".cctx/CBP-5/requirements.md", ".cctx/CBP-5/*.md"} {
		if err := runArchive(archiveCmd, []string{arg}); err == nil || !strings.Contains(err.Error(), "workspace") {
			t.Errorf("%s: expected workspace refusal, got %v", arg, err)
		}
	}
	if !exists(filepath.Join(ws, "requirements.md")) {
		t.Fatal("workspace touched")
	}

	writeFile(t, filepath.Join(h.projectDir, "v4-a.md"), "1")
	os.Symlink("v4-a.md", filepath.Join(h.projectDir, "v4-link.md"))
	if err := runArchive(archiveCmd, []string{"v4-*.md"}); err != nil {
		t.Fatalf("glob: %v", err)
	}
	if !exists(filepath.Join(h.projectDir, "v4-link.md")) {
		t.Error("symlink should be skipped, not archived")
	}
	writeFile(t, filepath.Join(h.projectDir, "v4-a.md"), "2")
	if err := runArchive(archiveCmd, []string{"v4-a.md"}); err != nil {
		t.Fatalf("second archive: %v", err)
	}
	day := time.Now().Format("2006-01-02") + "_test-project"
	names := strings.Join(archivedDirs(t, h), ",")
	if !strings.Contains(names, day+",") && !strings.HasSuffix(names, day) || !strings.Contains(names, day+"-1") {
		t.Errorf("expected %s and %s-1, got %s", day, day, names)
	}
}

func TestTicketComplete_DirectoryTicketUsesProjectGitInfo(t *testing.T) {
	h := newArchiveEnv(t)
	initGitRepo(t, h.projectDir)
	gitRun(t, h.projectDir, "checkout", "-q", "-b", "feature/CBP-6")
	addDirTicket(t, h, h.projectDir, "CBP-6", "active")

	// cwd (tempDir) is not a git repo; info must come from the project
	if err := runTicketComplete(ticketCompleteCmd, []string{"CBP-6"}); err != nil {
		t.Fatalf("complete: %v", err)
	}
	tk := archivedTicket(t, h, "CBP-6")
	if tk == nil || len(tk.Commits) != 1 || !strings.Contains(tk.Notes, "feature/CBP-6") {
		t.Fatalf("git info not recorded: %+v", tk)
	}
}

func TestArchive_AllActive(t *testing.T) {
	h := newArchiveEnv(t)
	addDirTicket(t, h, h.projectDir, "CBP-10", "active")
	addDirTicket(t, h, h.projectDir, "CBP-11", "abandoned")
	broken := addDirTicket(t, h, h.projectDir, "CBP-12", "active")
	os.RemoveAll(broken)
	cfg := h.GetConfig()
	cfg.Tickets.Active = append(cfg.Tickets.Active, config.Ticket{TicketID: "OLD-9", Status: "active"})
	h.configMgr.Save(cfg)

	archiveAllActive, archiveForce = true, true
	if err := runArchive(archiveCmd, nil); err == nil || !strings.Contains(err.Error(), "Archived 2 of 3") {
		t.Fatalf("expected partial failure summary, got %v", err)
	}
	cfg = h.GetConfig()
	if archivedTicket(t, h, "CBP-10") == nil || archivedTicket(t, h, "CBP-11") == nil {
		t.Error("healthy tickets should be archived")
	}
	if cfg.GetTicket("CBP-12", false) == nil || cfg.GetTicket("OLD-9", false) == nil {
		t.Error("failed and legacy tickets should stay active")
	}
	if tk := archivedTicket(t, h, "CBP-11"); tk.CompletedAt != nil {
		t.Error("abandoned ticket must not be completed")
	}
}

func TestArchive_AllActiveProjectFilter(t *testing.T) {
	h := newArchiveEnv(t)
	other := filepath.Join(h.tempDir, "other")
	h.AddProject("test-project", h.projectDir)
	addDirTicket(t, h, h.projectDir, "CBP-20", "active")
	addDirTicket(t, h, other, "CBP-21", "active")

	archiveAllActive, archiveForce, projectFlag = true, true, "test-project"
	if err := runArchive(archiveCmd, nil); err != nil {
		t.Fatalf("runArchive: %v", err)
	}
	if archivedTicket(t, h, "CBP-20") == nil || archivedTicket(t, h, "CBP-21") != nil {
		t.Error("project filter not applied")
	}
}

func TestArchive_InvalidFlagCombinations(t *testing.T) {
	h := newArchiveEnv(t)
	addDirTicket(t, h, h.projectDir, "CBP-30", "active")
	cases := []struct {
		name string
		set  func()
		args []string
	}{
		{"list with args", func() { archiveList = true }, []string{"x"}},
		{"list with all-active", func() { archiveList, archiveAllActive = true, true }, nil},
		{"all without list", func() { archiveListAll = true }, []string{"x"}},
		{"negative limit", func() { archiveList, archiveLimit = true, -1 }, nil},
		{"all-active with args", func() { archiveAllActive = true }, []string{"x"}},
		{"commits with all-active", func() { archiveAllActive, archiveCommits = true, "a" }, nil},
		{"commits on a path", func() { archiveCommits = "a" }, []string{"x.md"}},
		{"-t with args", func() { ticketFlag = "CBP-30" }, []string{"x"}},
		{"ticket mixed with args", func() {}, []string{"CBP-30", "x"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resetArchiveFlags()
			c.set()
			if err := runArchive(archiveCmd, c.args); err == nil {
				t.Error("expected error")
			}
		})
	}
	if h.GetConfig().GetTicket("CBP-30", false) == nil {
		t.Error("ticket changed by an invalid invocation")
	}
}

func TestArchive_NoArgsResolvesBranchLastSegment(t *testing.T) {
	h := newArchiveEnv(t)
	initGitRepo(t, h.projectDir)
	gitRun(t, h.projectDir, "checkout", "-q", "-b", "feature/CBP-40")
	addDirTicket(t, h, h.projectDir, "CBP-40", "active")
	os.Chdir(h.projectDir)

	archiveForce = true
	if err := runArchive(archiveCmd, nil); err != nil {
		t.Fatalf("runArchive: %v", err)
	}
	if archivedTicket(t, h, "CBP-40") == nil {
		t.Error("ticket from branch segment not archived")
	}
}

func TestArchiveList_ScopeOrderLimitAndBackfill(t *testing.T) {
	h := newArchiveEnv(t)
	repoA := filepath.Join(h.tempDir, "repoA")
	repoB := filepath.Join(h.tempDir, "repoB")
	os.MkdirAll(filepath.Join(repoA, "sub"), 0755)
	os.MkdirAll(repoB, 0755)
	initGitRepo(t, repoA)

	archived := filepath.Join(h.dataDir, "contexts", "_archived")
	for _, d := range []string{"2026-01-01_OLD-1", "2026-01-02_OLD-1-1", "2026-01-03_repoB", ".staging-abc"} {
		os.MkdirAll(filepath.Join(archived, d), 0755)
		writeFile(t, filepath.Join(archived, d, "f.md"), "x")
	}
	writeFile(t, filepath.Join(archived, ".DS_Store"), "")

	cfg := h.GetConfig()
	cfg.ManagedProjects = append(cfg.ManagedProjects, config.Project{ContextName: "repoB", ProjectPath: repoB})
	cfg.Tickets.Archived = append(cfg.Tickets.Archived, config.Ticket{TicketID: "OLD-1", Status: "completed", ArchivedPath: "contexts/_archived/2026-01-01_OLD-1"})
	cfg.Archives = append(cfg.Archives,
		config.ArchiveRecord{ArchiveDir: "contexts/_archived/x1", SourcePath: canon(repoA), ArchivedAt: time.Now().Add(-time.Hour), Kind: "path"},
		config.ArchiveRecord{ArchiveDir: "contexts/_archived/x2", SourcePath: canon(filepath.Join(repoA, "sub")), ArchivedAt: time.Now(), Kind: "path"},
	)
	h.configMgr.Save(cfg)

	added := backfillArchiveRecords(h.configMgr, cfg)
	if added != 3 {
		t.Fatalf("backfill added %d, want 3", added)
	}
	byDir := map[string]config.ArchiveRecord{}
	for _, r := range cfg.Archives {
		byDir[filepath.Base(r.ArchiveDir)] = r
	}
	if byDir["2026-01-01_OLD-1"].TicketID != "OLD-1" || byDir["2026-01-02_OLD-1-1"].TicketID != "OLD-1" {
		t.Errorf("ticket not linked by name: %+v", byDir)
	}
	if byDir["2026-01-03_repoB"].SourcePath != canon(repoB) {
		t.Errorf("project not inferred from dir name: %+v", byDir["2026-01-03_repoB"])
	}
	if backfillArchiveRecords(h.configMgr, cfg) != 0 {
		t.Error("backfill is not idempotent")
	}
	h.configMgr.Save(cfg)

	os.Chdir(filepath.Join(repoA, "sub"))
	archiveList = true
	out := captureStdout(t, func() {
		if err := runArchive(archiveCmd, nil); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "x1") || !strings.Contains(out, "x2") || strings.Contains(out, "repoB") {
		t.Errorf("scope should be repoA's git top-level:\n%s", out)
	}
	if strings.Index(out, "x2") > strings.Index(out, "x1") {
		t.Errorf("not newest first:\n%s", out)
	}

	archiveListAll, archiveLimit = true, 0
	out = captureStdout(t, func() { runArchive(archiveCmd, nil) })
	if !strings.Contains(out, "repoB") || !strings.Contains(out, "unknown") {
		t.Errorf("--all output:\n%s", out)
	}
	archiveLimit = 1
	out = captureStdout(t, func() { runArchive(archiveCmd, nil) })
	if strings.Count(out, "\n") != 2 {
		t.Errorf("--limit 1 output:\n%s", out)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = orig
	buf := make([]byte, 1<<16)
	n, _ := r.Read(buf)
	return string(buf[:n])
}

func TestArchive_MixedArgsRefusedBeforeAnythingMoves(t *testing.T) {
	h := newArchiveEnv(t)
	initGitRepo(t, h.projectDir)
	writeFile(t, filepath.Join(h.projectDir, "specs", "foo", "a.md"), "a")
	writeFile(t, filepath.Join(h.projectDir, "tracked.md"), "t")
	gitCommitFile(t, h.projectDir, "tracked.md")
	os.Chdir(h.projectDir)

	archiveForce = true
	if err := runArchive(archiveCmd, []string{"specs/foo", "tracked.md"}); err == nil {
		t.Fatal("expected refusal")
	}
	if !exists(filepath.Join(h.projectDir, "specs", "foo", "a.md")) || len(archivedDirs(t, h)) != 0 {
		t.Error("an earlier group was archived before a later one was refused")
	}
}

func TestArchive_AllActiveSaveFailureDoesNotLeak(t *testing.T) {
	h := newArchiveEnv(t)
	addDirTicket(t, h, h.projectDir, "CBP-50", "active")
	ws := addDirTicket(t, h, h.projectDir, "CBP-51", "active")
	addDirTicket(t, h, h.projectDir, "CBP-52", "active")

	calls := 0
	saveConfigFn = func(m *config.Manager, c *config.Config) error {
		calls++
		if calls == 2 {
			return errors.New("disk full")
		}
		return m.Save(c)
	}
	defer func() { saveConfigFn = func(m *config.Manager, c *config.Config) error { return m.Save(c) } }()

	archiveAllActive, archiveForce = true, true
	if err := runArchive(archiveCmd, nil); err == nil {
		t.Fatal("expected a failure summary")
	}
	cfg := h.GetConfig()
	if archivedTicket(t, h, "CBP-50") == nil || archivedTicket(t, h, "CBP-52") == nil {
		t.Error("other tickets should still be archived")
	}
	if tk := cfg.GetTicket("CBP-51", false); tk == nil || tk.Status != "active" || tk.ArchivedPath != "" {
		t.Errorf("failed ticket leaked state: %+v", tk)
	}
	if !exists(filepath.Join(ws, "requirements.md")) {
		t.Error("failed ticket's workspace was removed")
	}
	if len(cfg.Archives) != 2 {
		t.Errorf("records = %d, want 2", len(cfg.Archives))
	}
}

func TestArchiveTicket_LeftoversGoToNewDir(t *testing.T) {
	h := newArchiveEnv(t)
	ws := addDirTicket(t, h, h.projectDir, "CBP-60", "active")
	archiveForce = true
	if err := runArchive(archiveCmd, []string{"CBP-60"}); err != nil {
		t.Fatal(err)
	}
	first := archivedTicket(t, h, "CBP-60").ArchivedPath

	writeFile(t, filepath.Join(ws, "late.md"), "written after archiving")
	if err := runArchive(archiveCmd, []string{"CBP-60"}); err != nil {
		t.Fatalf("leftovers: %v", err)
	}
	cfg := h.GetConfig()
	if got := archivedTicket(t, h, "CBP-60").ArchivedPath; got != first {
		t.Errorf("archived_path changed from %s to %s", first, got)
	}
	if len(cfg.Archives) != 2 || cfg.Archives[1].Kind != "ticket" || cfg.Archives[1].TicketID != "CBP-60" || !strings.HasSuffix(cfg.Archives[1].ArchiveDir, "_CBP-60-1") {
		t.Errorf("leftover record: %+v", cfg.Archives)
	}
	if exists(ws) {
		t.Error("leftover workspace should be archived")
	}
}

func TestArchive_RefusesGitInternals(t *testing.T) {
	h := newArchiveEnv(t)
	initGitRepo(t, h.projectDir)
	os.Chdir(h.projectDir)
	archiveForce = true
	for _, arg := range []string{".git", ".git/HEAD"} {
		if err := runArchive(archiveCmd, []string{arg}); err == nil {
			t.Errorf("%s: expected refusal", arg)
		}
	}
	if !exists(filepath.Join(h.projectDir, ".git", "HEAD")) {
		t.Fatal(".git was touched")
	}
}

func TestRemovedTicketArchiveCommandsAreUnknown(t *testing.T) {
	newArchiveEnv(t)
	for _, args := range [][]string{{"ticket", "archive", "CBP-1"}, {"ticket", "archive-all"}} {
		rootCmd.SetArgs(args)
		err := rootCmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "unknown command") {
			t.Errorf("%v: %v", args, err)
		}
	}
	rootCmd.SetArgs([]string{"archive", "--limit", "5", "x.md"})
	if err := rootCmd.Execute(); err == nil || !strings.Contains(err.Error(), "only apply to --list") {
		t.Errorf("--limit without --list: %v", err)
	}
	rootCmd.SetArgs(nil)
}

func TestShellQuote(t *testing.T) {
	for in, want := range map[string]string{
		"CBP-1.md":      "CBP-1.md",
		"CBP-1;rm.md":   "'CBP-1;rm.md'",
		"CBP-1`id`.md":  "'CBP-1`id`.md'",
		"it's CBP-1.md": `'it'\''s CBP-1.md'`,
	} {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestArchiveTicket_SymlinkedChangelogTargetUntouched(t *testing.T) {
	h := newArchiveEnv(t)
	ws := addDirTicket(t, h, h.projectDir, "CBP-70", "active")
	outside := filepath.Join(h.tempDir, "outside.md")
	writeFile(t, outside, "do not touch")
	os.Remove(filepath.Join(ws, "changelog.md"))
	os.Symlink(outside, filepath.Join(ws, "changelog.md"))

	archiveForce = true
	if err := runArchive(archiveCmd, []string{"CBP-70"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(outside); string(got) != "do not touch" {
		t.Errorf("completion note written through a symlink: %q", got)
	}
}

func TestArchive_TrailingSlashSymlinkArgRefused(t *testing.T) {
	h := newArchiveEnv(t)
	real := filepath.Join(h.tempDir, "elsewhere")
	writeFile(t, filepath.Join(real, "a.md"), "a")
	link := filepath.Join(h.projectDir, "link")
	os.Symlink(real, link)

	archiveForce = true
	if err := runArchive(archiveCmd, []string{link + "/"}); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Errorf("expected symlink refusal, got %v", err)
	}
	if !exists(filepath.Join(real, "a.md")) {
		t.Fatal("symlink target was archived")
	}
}

func TestArchive_OverlappingArgsArchivedOnce(t *testing.T) {
	h := newArchiveEnv(t)
	writeFile(t, filepath.Join(h.projectDir, "docs", "a.md"), "a")
	os.Chdir(h.projectDir)
	archiveForce = true
	if err := runArchive(archiveCmd, []string{"docs", "docs/*.md"}); err != nil {
		t.Fatalf("overlapping args: %v", err)
	}
	if n := len(archivedDirs(t, h)); n != 1 {
		t.Errorf("archive dirs = %d, want 1", n)
	}
}

func TestArchiveTicket_ActiveWithStaleArchivedPathIsArchivedNormally(t *testing.T) {
	h := newArchiveEnv(t)
	addDirTicket(t, h, h.projectDir, "CBP-71", "active")
	cfg := h.GetConfig()
	for i := range cfg.Tickets.Active {
		if cfg.Tickets.Active[i].TicketID == "CBP-71" {
			cfg.Tickets.Active[i].ArchivedPath = "contexts/_archived/2026-01-01_CBP-71"
		}
	}
	h.configMgr.Save(cfg)

	archiveForce = true
	if err := runArchive(archiveCmd, []string{"CBP-71"}); err != nil {
		t.Fatal(err)
	}
	if tk := archivedTicket(t, h, "CBP-71"); tk == nil || tk.Status != "completed" {
		t.Errorf("ticket should be completed and moved to archived: %+v", tk)
	}
}
