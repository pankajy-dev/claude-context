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
)

func newEngineEnv(t *testing.T) *TestHelper {
	t.Helper()
	h := NewTestHelper(t)
	t.Cleanup(h.Cleanup)
	if err := os.Chdir(h.tempDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	dataDir = h.dataDir
	dryRun = false
	return h
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func archivedDirs(t *testing.T, h *TestHelper) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(h.dataDir, "contexts", "_archived"))
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func TestArchiveItems_DirectoryTreeCopiedVerifiedAndRemoved(t *testing.T) {
	h := newEngineEnv(t)
	src := filepath.Join(h.projectDir, "specs", "login")
	writeFile(t, filepath.Join(src, "plan.md"), "plan")
	writeFile(t, filepath.Join(src, "nested", "deep.md"), "deep")
	if err := os.Symlink("plan.md", filepath.Join(src, "link.md")); err != nil {
		t.Fatal(err)
	}

	final, err := archiveItems(h.configMgr, []archiveItem{{Src: src, RelDest: "."}}, archiveOpts{
		Name:   "login",
		Record: config.ArchiveRecord{Kind: "path", SourcePath: canon(src)},
	})
	if err != nil {
		t.Fatalf("archiveItems: %v", err)
	}

	if got, _ := os.ReadFile(filepath.Join(final, "nested", "deep.md")); string(got) != "deep" {
		t.Errorf("nested file content = %q", got)
	}
	if target, err := os.Readlink(filepath.Join(final, "link.md")); err != nil || target != "plan.md" {
		t.Errorf("symlink not recreated: %q %v", target, err)
	}
	if exists(src) {
		t.Errorf("source dir %s still exists", src)
	}
	if !strings.HasSuffix(filepath.Base(final), "_login") {
		t.Errorf("archive dir name = %s", filepath.Base(final))
	}

	cfg := h.GetConfig()
	if len(cfg.Archives) != 1 {
		t.Fatalf("records = %d, want 1", len(cfg.Archives))
	}
	rec := cfg.Archives[0]
	if rec.FileCount != 3 || rec.Kind != "path" || rec.SourcePath != canon(filepath.Join(h.projectDir, "specs", "login")) {
		t.Errorf("unexpected record %+v", rec)
	}
	if rec.ArchiveDir != filepath.Join("contexts", "_archived", filepath.Base(final)) {
		t.Errorf("record dir = %s", rec.ArchiveDir)
	}
}

func TestArchiveItems_FailedCopyLeavesSourcesIntact(t *testing.T) {
	h := newEngineEnv(t)
	src := filepath.Join(h.projectDir, "work")
	writeFile(t, filepath.Join(src, "a.md"), "a")
	writeFile(t, filepath.Join(src, "b.md"), "b")

	calls := 0
	copyEntryFn = func(s, d string) error {
		calls++
		if calls == 2 {
			return errors.New("disk full")
		}
		return copyEntry(s, d)
	}
	defer func() { copyEntryFn = copyEntry }()

	if _, err := archiveItems(h.configMgr, []archiveItem{{Src: src, RelDest: "."}}, archiveOpts{Name: "work"}); err == nil {
		t.Fatal("expected error")
	}
	if !exists(filepath.Join(src, "a.md")) || !exists(filepath.Join(src, "b.md")) {
		t.Error("sources were removed after a failed copy")
	}
	if names := archivedDirs(t, h); len(names) != 0 {
		t.Errorf("archive root not empty: %v", names)
	}
}

func TestArchiveItems_CorruptCopyIsCaughtByVerification(t *testing.T) {
	h := newEngineEnv(t)
	src := filepath.Join(h.projectDir, "work.md")
	writeFile(t, src, "original")

	copyEntryFn = func(s, d string) error {
		return os.WriteFile(d, []byte("corrupt!"), 0644)
	}
	defer func() { copyEntryFn = copyEntry }()

	if _, err := archiveItems(h.configMgr, []archiveItem{{Src: src, RelDest: "work.md"}}, archiveOpts{Name: "x"}); err == nil {
		t.Fatal("expected verification error")
	}
	if !exists(src) {
		t.Error("source removed despite failed verification")
	}
	if names := archivedDirs(t, h); len(names) != 0 {
		t.Errorf("archive root not empty: %v", names)
	}
}

func TestArchiveItems_Refusals(t *testing.T) {
	h := newEngineEnv(t)
	cfg := h.GetConfig()
	ws := filepath.Join(h.projectDir, ".cctx", "CBP-9")
	writeFile(t, filepath.Join(ws, "plan.md"), "p")
	cfg.Tickets.Active = append(cfg.Tickets.Active, config.Ticket{TicketID: "CBP-9", Status: "active", WorkspacePath: canon(ws)})
	h.configMgr.Save(cfg)

	writeFile(t, filepath.Join(h.projectDir, "real.md"), "r")
	os.Symlink(filepath.Join(h.projectDir, "real.md"), filepath.Join(h.projectDir, "link.md"))
	writeFile(t, filepath.Join(h.projectDir, "nested", ".cctx", "x.md"), "x")
	writeFile(t, filepath.Join(h.projectDir, "clone", ".git", "HEAD"), "ref")
	writeFile(t, filepath.Join(h.projectDir, "one", "Notes.md"), "1")
	writeFile(t, filepath.Join(h.projectDir, "two", "notes.md"), "2")
	writeFile(t, filepath.Join(h.dataDir, "stray.md"), "s")

	tests := []struct {
		name  string
		items []archiveItem
		want  string
	}{
		{"missing source", []archiveItem{{Src: filepath.Join(h.projectDir, "nope"), RelDest: "."}}, "does not exist"},
		{"contains cwd", []archiveItem{{Src: h.tempDir, RelDest: "."}}, "run this from outside"},
		{"inside data dir", []archiveItem{{Src: filepath.Join(h.dataDir, "stray.md"), RelDest: "stray.md"}}, "data directory"},
		{"workspace root", []archiveItem{{Src: filepath.Join(h.projectDir, ".cctx"), RelDest: "."}}, "archive the tickets inside it"},
		{"symlink argument", []archiveItem{{Src: filepath.Join(h.projectDir, "link.md"), RelDest: "link.md"}}, "symlink"},
		{"nested git repo", []archiveItem{{Src: filepath.Join(h.projectDir, "clone"), RelDest: "."}}, "git repository"},
		{"nested workspace", []archiveItem{{Src: filepath.Join(h.projectDir, "nested"), RelDest: "."}}, ".cctx workspace"},
		{"case-insensitive collision", []archiveItem{
			{Src: filepath.Join(h.projectDir, "one"), RelDest: "."},
			{Src: filepath.Join(h.projectDir, "two"), RelDest: "."},
		}, "would both be archived"},
		{"active workspace", []archiveItem{{Src: filepath.Join(ws, "plan.md"), RelDest: "plan.md"}}, "ticket CBP-9's workspace"},
		{"contains active workspace", []archiveItem{{Src: filepath.Join(h.projectDir, ".cctx", "CBP-9"), RelDest: "."}}, "ticket CBP-9's workspace"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before, _ := os.ReadFile(filepath.Join(h.dataDir, "config.json"))
			_, err := archiveItems(h.configMgr, tt.items, archiveOpts{Name: "x"})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want %q", err, tt.want)
			}
			after, _ := os.ReadFile(filepath.Join(h.dataDir, "config.json"))
			if string(before) != string(after) {
				t.Error("config changed after a refusal")
			}
			for _, it := range tt.items {
				if strings.Contains(it.Src, "nope") {
					continue
				}
				if !exists(it.Src) {
					t.Errorf("%s removed after a refusal", it.Src)
				}
			}
			if names := archivedDirs(t, h); len(names) != 0 {
				t.Errorf("archive root not empty: %v", names)
			}
		})
	}
}

func TestArchiveItems_OwnTicketWorkspaceAllowed(t *testing.T) {
	h := newEngineEnv(t)
	ws := filepath.Join(h.projectDir, ".cctx", "CBP-9")
	writeFile(t, filepath.Join(ws, "plan.md"), "p")
	cfg := h.GetConfig()
	cfg.Tickets.Active = append(cfg.Tickets.Active, config.Ticket{TicketID: "CBP-9", Status: "active", WorkspacePath: canon(ws)})
	h.configMgr.Save(cfg)

	if _, err := archiveItems(h.configMgr, []archiveItem{{Src: ws, RelDest: "."}}, archiveOpts{Name: "CBP-9", OwnTicketID: "CBP-9"}); err != nil {
		t.Fatalf("archiveItems: %v", err)
	}
	if exists(ws) {
		t.Error("workspace dir should be removed")
	}
	if !exists(filepath.Join(h.projectDir, ".cctx")) {
		t.Error("parent .cctx should be left in place")
	}
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	if !gitAvailable() {
		t.Skip("git not installed")
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"-c", "user.email=t@example.com", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
}

func gitCommitFile(t *testing.T, dir, rel string) {
	t.Helper()
	for _, args := range [][]string{
		{"add", "--", rel},
		{"-c", "user.email=t@example.com", "-c", "user.name=t", "commit", "-q", "-m", "add"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
}

func TestArchiveItems_GitTrackedFilesRefused(t *testing.T) {
	h := newEngineEnv(t)
	initGitRepo(t, h.projectDir)

	tracked := filepath.Join(h.projectDir, "docs")
	writeFile(t, filepath.Join(tracked, "a[1].md"), "tracked")
	writeFile(t, filepath.Join(tracked, "b.md"), "untracked")
	gitCommitFile(t, h.projectDir, "docs/a[1].md")

	_, err := archiveItems(h.configMgr, []archiveItem{{Src: tracked, RelDest: "."}}, archiveOpts{Name: "docs"})
	if err == nil || !strings.Contains(err.Error(), "git-tracked") || !strings.Contains(err.Error(), "a[1].md") {
		t.Fatalf("err = %v, want git-tracked refusal naming a[1].md", err)
	}

	untracked := filepath.Join(h.projectDir, "scratch")
	writeFile(t, filepath.Join(untracked, "notes.md"), "n")
	if _, err := archiveItems(h.configMgr, []archiveItem{{Src: untracked, RelDest: "."}}, archiveOpts{Name: "scratch"}); err != nil {
		t.Fatalf("untracked dir should archive: %v", err)
	}
}

func TestArchiveItems_NonGitDirectoryArchives(t *testing.T) {
	h := newEngineEnv(t)
	if hasGitAncestor(h.projectDir) {
		t.Skip("temp dir is inside a git repo")
	}
	src := filepath.Join(h.projectDir, "plain")
	writeFile(t, filepath.Join(src, "a.md"), "a")
	if _, err := archiveItems(h.configMgr, []archiveItem{{Src: src, RelDest: "."}}, archiveOpts{Name: "plain"}); err != nil {
		t.Fatalf("archiveItems in non-git dir: %v", err)
	}
}

func TestArchiveItems_DryRunChangesNothing(t *testing.T) {
	h := newEngineEnv(t)
	src := filepath.Join(h.projectDir, "work")
	writeFile(t, filepath.Join(src, "a.md"), "a")
	before, _ := os.ReadFile(filepath.Join(h.dataDir, "config.json"))

	dryRun = true
	defer func() { dryRun = false }()
	if _, err := archiveItems(h.configMgr, []archiveItem{{Src: src, RelDest: "."}}, archiveOpts{Name: "work"}); err != nil {
		t.Fatalf("dry run: %v", err)
	}

	after, _ := os.ReadFile(filepath.Join(h.dataDir, "config.json"))
	if string(before) != string(after) || !exists(filepath.Join(src, "a.md")) || len(archivedDirs(t, h)) != 0 {
		t.Error("dry run changed state")
	}
}

func TestArchiveItems_NeverReusesExistingArchiveDir(t *testing.T) {
	h := newEngineEnv(t)
	existing := filepath.Join(h.dataDir, "contexts", "_archived", time.Now().Format("2006-01-02")+"_notes")
	writeFile(t, filepath.Join(existing, "a.md"), "old")

	src := filepath.Join(h.projectDir, "a.md")
	writeFile(t, src, "new")
	final, err := archiveItems(h.configMgr, []archiveItem{{Src: src, RelDest: "a.md"}}, archiveOpts{Name: "notes"})
	if err != nil {
		t.Fatalf("archiveItems: %v", err)
	}
	if final != existing+"-1" {
		t.Errorf("final = %s, want %s-1", final, existing)
	}
	if got, _ := os.ReadFile(filepath.Join(existing, "a.md")); string(got) != "old" {
		t.Error("existing archive was modified")
	}
}

func TestArchiveItems_FileChangedBeforeDeleteIsKept(t *testing.T) {
	h := newEngineEnv(t)
	src := filepath.Join(h.projectDir, "work")
	writeFile(t, filepath.Join(src, "changelog.md"), "v1")
	writeFile(t, filepath.Join(src, "plan.md"), "plan")

	beforeDeleteHook = func() {
		writeFile(t, filepath.Join(src, "changelog.md"), "v2 with more")
	}
	defer func() { beforeDeleteHook = nil }()

	final, err := archiveItems(h.configMgr, []archiveItem{{Src: src, RelDest: "."}}, archiveOpts{Name: "work"})
	if err != nil {
		t.Fatalf("archiveItems: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(src, "changelog.md")); string(got) != "v2 with more" {
		t.Errorf("changed file not kept: %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(final, "changelog.md")); string(got) != "v1" {
		t.Errorf("archive should hold v1, got %q", got)
	}
	if exists(filepath.Join(src, "plan.md")) {
		t.Error("unchanged file should be removed")
	}
}

func TestArchiveItems_SaveFailureIsNoOp(t *testing.T) {
	h := newEngineEnv(t)
	src := filepath.Join(h.projectDir, "work")
	writeFile(t, filepath.Join(src, "a.md"), "a")

	saveConfigFn = func(*config.Manager, *config.Config) error { return errors.New("read-only fs") }
	defer func() { saveConfigFn = func(m *config.Manager, c *config.Config) error { return m.Save(c) } }()

	if _, err := archiveItems(h.configMgr, []archiveItem{{Src: src, RelDest: "."}}, archiveOpts{Name: "work"}); err == nil {
		t.Fatal("expected save error")
	}
	if !exists(filepath.Join(src, "a.md")) {
		t.Error("source removed despite save failure")
	}
	if names := archivedDirs(t, h); len(names) != 0 {
		t.Errorf("archive dir left behind: %v", names)
	}
}

func TestValidTicketID(t *testing.T) {
	for id, want := range map[string]bool{
		"CBP-1": true, "fix_login.2": true,
		"../x": false, "a/b": false, "feature/CBP-1": false, "CBP-1.md": false, ".hidden": false, "": false,
	} {
		if got := validTicketID(id); got != want {
			t.Errorf("validTicketID(%q) = %v, want %v", id, got, want)
		}
	}
}

func TestArchiveItems_ProtectedAncestorsRefused(t *testing.T) {
	h := newEngineEnv(t)
	home := filepath.Join(h.tempDir, "home")
	os.MkdirAll(home, 0755)
	t.Setenv("HOME", home)
	initGitRepo(t, h.projectDir)
	sub := filepath.Join(h.projectDir, "sub")
	os.MkdirAll(sub, 0755)
	os.Chdir(sub)

	for name, src := range map[string]string{"root": "/", "home": home, "git top-level": h.projectDir} {
		if _, err := archiveItems(h.configMgr, []archiveItem{{Src: src, RelDest: "."}}, archiveOpts{Name: "x"}); err == nil {
			t.Errorf("%s: expected refusal", name)
		}
	}
	if !exists(home) || !exists(sub) {
		t.Fatal("protected directory removed")
	}
}

func TestArchiveItems_GitMissingOrBrokenRefusedInsideRepo(t *testing.T) {
	h := newEngineEnv(t)

	broken := filepath.Join(h.projectDir, "broken")
	writeFile(t, filepath.Join(broken, ".git"), "gitdir: /nonexistent/repo\n")
	writeFile(t, filepath.Join(broken, "notes", "a.md"), "a")
	if gitAvailable() {
		_, err := archiveItems(h.configMgr, []archiveItem{{Src: filepath.Join(broken, "notes"), RelDest: "."}}, archiveOpts{Name: "x"})
		if err == nil || !strings.Contains(err.Error(), "cannot check git-tracked files") {
			t.Errorf("broken repo: %v", err)
		}
	}

	t.Setenv("PATH", t.TempDir())
	_, err := archiveItems(h.configMgr, []archiveItem{{Src: filepath.Join(broken, "notes"), RelDest: "."}}, archiveOpts{Name: "x"})
	if !errors.Is(err, errGitMissing) {
		t.Errorf("git missing: %v", err)
	}
	if !exists(filepath.Join(broken, "notes", "a.md")) {
		t.Fatal("source removed")
	}
}
