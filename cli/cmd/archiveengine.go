package cmd

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/pankaj/claude-context/internal/config"
)

// Every command that moves files into contexts/_archived/ goes through
// archiveItems. It refuses anything risky up front, copies into a staging dir,
// verifies the copy, records it in config and only then deletes the sources.
// It never writes into an archive dir that existed before the call.

// archiveItem is one source to archive. A directory item copies the
// directory's contents under RelDest ("." = archive root) and removes the
// directory itself afterwards.
type archiveItem struct {
	Src     string
	RelDest string
}

type archiveOpts struct {
	Name        string
	Record      config.ArchiveRecord
	OwnTicketID string // ticket mode: the ticket whose workspace is being archived
	Mutate      func(cfg *config.Config, archiveRel string) error
	Decorate    func(stagingDir string) error
}

// Test seams
var (
	copyEntryFn      = copyEntry
	beforeDeleteHook func()
	saveConfigFn     = func(m *config.Manager, c *config.Config) error { return m.Save(c) }
)

// Refusals the user can fix by changing how they run the command, rather than
// by changing what they archive.
var (
	errCwdInside  = errors.New("run this from outside it")
	errGitMissing = errors.New("cannot verify git-tracked files: git not found")

	errOutsideWorkTree = errors.New("outside a git work tree")
)

var ticketIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// validTicketID reports whether id is safe to use as a path component.
func validTicketID(id string) bool {
	return ticketIDPattern.MatchString(id) &&
		!strings.Contains(id, "..") &&
		!strings.HasSuffix(strings.ToLower(id), ".md")
}

// canon returns an absolute, symlink-resolved path. For a path that does not
// exist yet, the deepest existing ancestor is resolved and the rest re-appended.
func canon(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return filepath.Clean(p)
	}
	rest := ""
	for cur := abs; ; {
		if resolved, err := filepath.EvalSymlinks(cur); err == nil {
			return filepath.Join(resolved, rest)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return abs
		}
		rest = filepath.Join(filepath.Base(cur), rest)
		cur = parent
	}
}

// isWithin reports whether path equals root or lies below it. macOS volumes
// are case-insensitive by default, so /users/x and /Users/x are the same dir.
func isWithin(path, root string) bool {
	if runtime.GOOS == "darwin" {
		path, root = strings.ToLower(path), strings.ToLower(root)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cctx")
}

// git helpers. fsmonitor is disabled so a repo's config can't run commands.

func gitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// gitCommand runs git without inheriting GIT_DIR and friends, which would make
// it answer about some other repository.
func gitCommand(args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	for _, kv := range os.Environ() {
		switch strings.SplitN(kv, "=", 2)[0] {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR":
			continue
		}
		cmd.Env = append(cmd.Env, kv)
	}
	return cmd
}

func gitOutput(dir string, args ...string) (string, error) {
	full := append([]string{"-c", "core.fsmonitor=false", "-C", dir}, args...)
	out, err := gitCommand(full...).Output()
	return strings.TrimSpace(string(out)), err
}

func gitInsideWorkTree(dir string) bool {
	inside, err := gitWorkTreeState(dir)
	return err == nil && inside
}

// gitWorkTreeState distinguishes "not a repo" from git failing to answer.
func gitWorkTreeState(dir string) (bool, error) {
	out, err := gitOutput(dir, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return false, err
	}
	return out == "true", nil
}

func gitTopLevel(dir string) string {
	out, err := gitOutput(dir, "rev-parse", "--show-toplevel")
	if err != nil || out == "" {
		return ""
	}
	return canon(out)
}

func gitTrackedFiles(dir, src string) ([]string, error) {
	out, err := gitCommand("--literal-pathspecs", "-c", "core.fsmonitor=false",
		"-C", dir, "ls-files", "-z", "--", src).Output()
	if err != nil {
		return nil, err
	}
	files := []string{}
	for _, f := range bytes.Split(out, []byte{0}) {
		if len(f) > 0 {
			files = append(files, string(f))
		}
	}
	return files, nil
}

func hasGitAncestor(p string) bool {
	for dir := p; ; {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}

// archiveRefusals checks every item before anything is written.
func archiveRefusals(cfg *config.Config, dataDir string, items []archiveItem, opts archiveOpts) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	cwd = canon(cwd)

	dataRoots := []string{canon(dataDir)}
	if def := defaultDataDir(); def != "" {
		dataRoots = append(dataRoots, canon(def))
	}
	protected := append([]string{"/", cwd}, dataRoots...)
	if home, err := os.UserHomeDir(); err == nil {
		protected = append(protected, canon(home))
	}
	haveGit := gitAvailable()
	if haveGit {
		if top := gitTopLevel(cwd); top != "" {
			protected = append(protected, top)
		}
	}

	destinations := map[string]string{}
	trackedCache := map[string]error{}

	for _, it := range items {
		info, err := os.Lstat(it.Src)
		if err != nil {
			return fmt.Errorf("refusing to archive %s: it does not exist", it.Src)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to archive %s: it is a symlink, pass the real path", it.Src)
		}
		src := canon(it.Src)

		if isWithin(cwd, src) {
			return fmt.Errorf("refusing to archive %s: %w", src, errCwdInside)
		}
		for _, p := range protected {
			if isWithin(p, src) {
				return fmt.Errorf("refusing to archive %s: it contains %s", src, p)
			}
		}
		for _, root := range dataRoots {
			if isWithin(src, root) {
				return fmt.Errorf("refusing to archive %s: it is inside the cctx data directory", src)
			}
		}
		if filepath.Base(src) == ".cctx" {
			return fmt.Errorf("refusing to archive %s: archive the tickets inside it instead", src)
		}
		for _, part := range strings.Split(src, string(filepath.Separator)) {
			if part == ".git" {
				return fmt.Errorf("refusing to archive %s: it is part of a git repository's internals", src)
			}
		}
		for _, t := range cfg.Tickets.Active {
			if t.WorkspacePath == "" || t.TicketID == opts.OwnTicketID {
				continue
			}
			ws := canon(t.WorkspacePath)
			if isWithin(src, ws) || isWithin(ws, src) {
				return fmt.Errorf("refusing to archive %s: it is part of ticket %s's workspace; run `cctx archive %s`", src, t.TicketID, t.TicketID)
			}
		}

		rels := []string{"."}
		if info.IsDir() {
			rels = nil
			err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if path == src {
					return nil
				}
				switch d.Name() {
				case ".cctx":
					return fmt.Errorf("refusing to archive %s: it contains a .cctx workspace at %s", src, path)
				case ".git":
					return fmt.Errorf("refusing to archive %s: it contains a git repository at %s", src, path)
				}
				if d.Type()&^(fs.ModeDir|fs.ModeSymlink) != 0 {
					return fmt.Errorf("refusing to archive %s: unsupported file type at %s", src, path)
				}
				if !d.IsDir() {
					rel, _ := filepath.Rel(src, path)
					rels = append(rels, rel)
				}
				return nil
			})
			if err != nil {
				return err
			}
		} else if !info.Mode().IsRegular() {
			return fmt.Errorf("refusing to archive %s: unsupported file type", src)
		}

		for _, rel := range rels {
			dest := filepath.Join(it.RelDest, rel)
			key := strings.ToLower(dest)
			if prev, ok := destinations[key]; ok {
				return fmt.Errorf("refusing to archive: %s and %s would both be archived as %s", prev, filepath.Join(src, rel), dest)
			}
			destinations[key] = filepath.Join(src, rel)
		}

		gitDir := src
		if !info.IsDir() {
			gitDir = filepath.Dir(src)
		}
		if haveGit {
			stateErr, cached := trackedCache[gitDir]
			inside := false
			if !cached {
				inside, stateErr = gitWorkTreeState(gitDir)
				if stateErr == nil && !inside {
					stateErr = errOutsideWorkTree
				}
				trackedCache[gitDir] = stateErr
			}
			inside = stateErr == nil
			// git failing inside what looks like a repo must not read as "untracked"
			if stateErr != nil && stateErr != errOutsideWorkTree && hasGitAncestor(src) {
				return fmt.Errorf("refusing to archive %s: cannot check git-tracked files: %v", src, stateErr)
			}
			if inside {
				tracked, err := gitTrackedFiles(gitDir, src)
				if err != nil {
					return fmt.Errorf("refusing to archive %s: cannot check git-tracked files: %w", src, err)
				}
				if len(tracked) > 0 {
					if len(tracked) > 10 {
						tracked = append(tracked[:10], "…")
					}
					return fmt.Errorf("refusing to archive %s: it contains git-tracked files: %s", src, strings.Join(tracked, ", "))
				}
			}
		} else if hasGitAncestor(src) {
			return fmt.Errorf("refusing to archive %s: %w", src, errGitMissing)
		}
	}
	return nil
}

type copiedEntry struct {
	src, dst string
	symlink  bool
	size     int64
	mtime    time.Time
}

// copyEntry copies one regular file or symlink without following symlinks.
func copyEntry(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		return os.Symlink(target, dst)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("unsupported file type: %s", src)
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func fileSHA256(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

// copyTree copies src (file or directory) into dstRoot/relDest.
func copyTree(src, dstRoot, relDest string) ([]copiedEntry, []string, error) {
	var copied []copiedEntry
	var dirs []string

	add := func(path, dst string) error {
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if err := copyEntryFn(path, dst); err != nil {
			return fmt.Errorf("failed to copy %s: %w", path, err)
		}
		copied = append(copied, copiedEntry{
			src: path, dst: dst,
			symlink: info.Mode()&os.ModeSymlink != 0,
			size:    info.Size(), mtime: info.ModTime(),
		})
		return nil
	}

	info, err := os.Lstat(src)
	if err != nil {
		return nil, nil, err
	}
	if !info.IsDir() {
		return copied, nil, add(src, filepath.Join(dstRoot, relDest))
	}

	err = filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		dst := filepath.Join(dstRoot, relDest, rel)
		if d.IsDir() {
			dirs = append(dirs, path)
			info, err := d.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(dst, info.Mode().Perm()|0700)
		}
		return add(path, dst)
	})
	return copied, dirs, err
}

func verifyCopies(copied []copiedEntry) error {
	for _, c := range copied {
		if c.symlink {
			want, err1 := os.Readlink(c.src)
			got, err2 := os.Readlink(c.dst)
			if err1 != nil || err2 != nil || want != got {
				return fmt.Errorf("verification failed for %s", c.src)
			}
			continue
		}
		srcInfo, err1 := os.Lstat(c.src)
		dstInfo, err2 := os.Lstat(c.dst)
		if err1 != nil || err2 != nil || srcInfo.Size() != dstInfo.Size() {
			return fmt.Errorf("verification failed for %s", c.src)
		}
		want, err1 := fileSHA256(c.src)
		got, err2 := fileSHA256(c.dst)
		if err1 != nil || err2 != nil || !bytes.Equal(want, got) {
			return fmt.Errorf("verification failed for %s", c.src)
		}
	}
	return nil
}

// reserveArchiveDir creates a fresh <date>_<name>[-n] dir under root.
func reserveArchiveDir(root, name string) (string, error) {
	base := filepath.Join(root, fmt.Sprintf("%s_%s", time.Now().Format("2006-01-02"), name))
	for i := 0; ; i++ {
		candidate := base
		if i > 0 {
			candidate = fmt.Sprintf("%s-%d", base, i)
		}
		err := os.Mkdir(candidate, 0755)
		if err == nil {
			return candidate, nil
		}
		if !os.IsExist(err) {
			return "", fmt.Errorf("failed to create archive directory: %w", err)
		}
	}
}

func countArchiveEntries(dir string) int {
	n := 0
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			n++
		}
		return nil
	})
	return n
}

// archiveItems archives items into a new dir and returns its absolute path.
// In dry-run mode it only reports the plan and returns "".
func archiveItems(cfgMgr *config.Manager, items []archiveItem, opts archiveOpts) (string, error) {
	if len(items) == 0 {
		return "", fmt.Errorf("nothing to archive")
	}
	cfg, err := cfgMgr.Load()
	if err != nil {
		return "", fmt.Errorf("failed to load config: %w", err)
	}
	if err := archiveRefusals(cfg, cfgMgr.GetRepoRoot(), items, opts); err != nil {
		return "", err
	}
	// Copy and delete exactly the paths that were checked
	checked := make([]archiveItem, len(items))
	for i, it := range items {
		checked[i] = archiveItem{Src: canon(it.Src), RelDest: it.RelDest}
	}
	items = checked

	name := opts.Name
	if !ticketIDPattern.MatchString(name) {
		cwd, _ := os.Getwd()
		name = filepath.Base(cwd)
	}
	archivedRoot := filepath.Join(cfgMgr.GetContextsPath(), "_archived")

	if dryRun {
		for _, it := range items {
			dryRunMsg(fmt.Sprintf("Would archive %s → _archived/%s_%s/", it.Src, time.Now().Format("2006-01-02"), name))
		}
		return "", nil
	}

	if err := os.MkdirAll(archivedRoot, 0755); err != nil {
		return "", fmt.Errorf("failed to create archive root: %w", err)
	}
	staging, err := os.MkdirTemp(archivedRoot, ".staging-")
	if err != nil {
		return "", fmt.Errorf("failed to create staging directory: %w", err)
	}
	defer os.RemoveAll(staging)

	var copied []copiedEntry
	var dirs []string
	for _, it := range items {
		c, d, err := copyTree(it.Src, staging, it.RelDest)
		if err != nil {
			return "", err
		}
		copied = append(copied, c...)
		dirs = append(dirs, d...)
	}
	if err := verifyCopies(copied); err != nil {
		return "", err
	}
	if opts.Decorate != nil {
		if err := opts.Decorate(staging); err != nil {
			return "", err
		}
	}

	final, err := reserveArchiveDir(archivedRoot, name)
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(staging)
	if err != nil {
		os.RemoveAll(final)
		return "", err
	}
	for _, e := range entries {
		if err := os.Rename(filepath.Join(staging, e.Name()), filepath.Join(final, e.Name())); err != nil {
			os.RemoveAll(final)
			return "", fmt.Errorf("failed to move archive into place: %w", err)
		}
	}

	rel := filepath.Join("contexts", "_archived", filepath.Base(final))
	cfg, err = cfgMgr.Load()
	if err != nil {
		os.RemoveAll(final)
		return "", fmt.Errorf("failed to load config: %w", err)
	}
	record := opts.Record
	record.ArchiveDir = rel
	record.ArchivedAt = time.Now()
	record.FileCount = countArchiveEntries(final)
	cfg.Archives = append(cfg.Archives, record)
	if opts.Mutate != nil {
		if err := opts.Mutate(cfg, rel); err != nil {
			os.RemoveAll(final)
			return "", err
		}
	}
	if err := saveConfigFn(cfgMgr, cfg); err != nil {
		os.RemoveAll(final)
		return "", fmt.Errorf("failed to save config, nothing was archived: %w", err)
	}

	removeArchivedSources(copied, dirs)
	return final, nil
}

// removeArchivedSources deletes what was archived. A file that changed since it
// was copied is kept; problems are reported but never undo the archive.
func removeArchivedSources(copied []copiedEntry, dirs []string) {
	if beforeDeleteHook != nil {
		beforeDeleteHook()
	}
	left := []string{}
	for _, c := range copied {
		if !c.symlink {
			info, err := os.Lstat(c.src)
			if err != nil {
				continue
			}
			if info.Size() != c.size || !info.ModTime().Equal(c.mtime) {
				warningMsg(fmt.Sprintf("changed during archive; kept %s (archive holds the earlier version)", c.src))
				left = append(left, c.src)
				continue
			}
		}
		if err := os.Remove(c.src); err != nil {
			left = append(left, c.src)
		}
	}

	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	for _, d := range dirs {
		if err := os.Remove(d); err != nil && !os.IsNotExist(err) {
			left = append(left, d)
		}
	}
	if len(left) > 0 {
		warningMsg(fmt.Sprintf("left in place: %s; re-run `cctx archive` on them later", strings.Join(left, ", ")))
	}
}
