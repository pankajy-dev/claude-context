package cmd

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// TestMatchArchiveFiles_TicketIDFallback verifies that a bare ticket ID
// pattern (no extension, no glob metacharacters) picks up every file that
// contains it as a whole word, regardless of extension or surrounding
// prefix/suffix text.
func TestMatchArchiveFiles_TicketIDFallback(t *testing.T) {
	dir := t.TempDir()

	want := []string{
		"CBP-YYYY.md",
		"CBP-YYYY.txt",
		"prefix-CBP-YYYY-suffix.md",
	}
	for _, name := range want {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("content"), 0644); err != nil {
			t.Fatalf("failed to create fixture %s: %v", name, err)
		}
	}

	// A file that merely shares a prefix (CBP-YYYY0) must not match, since
	// IsTicketFileName requires a whole-word match.
	if err := os.WriteFile(filepath.Join(dir, "CBP-YYYY0.md"), []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create fixture: %v", err)
	}

	got, err := matchArchiveFiles(dir, []string{"CBP-YYYY"})
	if err != nil {
		t.Fatalf("matchArchiveFiles returned error: %v", err)
	}

	gotNames := make([]string, len(got))
	for i, f := range got {
		gotNames[i] = filepath.Base(f)
	}
	sort.Strings(gotNames)
	sort.Strings(want)

	if len(gotNames) != len(want) {
		t.Fatalf("matched %v, want %v", gotNames, want)
	}
	for i := range want {
		if gotNames[i] != want[i] {
			t.Errorf("matched %v, want %v", gotNames, want)
			break
		}
	}
}

// TestMatchArchiveFiles_ExactNameStillWorks verifies the pre-existing literal
// and glob matching paths keep working alongside the ticket ID fallback.
func TestMatchArchiveFiles_ExactNameStillWorks(t *testing.T) {
	dir := t.TempDir()

	for _, name := range []string{"CBP-YYYY.md", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("content"), 0644); err != nil {
			t.Fatalf("failed to create fixture %s: %v", name, err)
		}
	}

	got, err := matchArchiveFiles(dir, []string{"CBP-YYYY.md"})
	if err != nil {
		t.Fatalf("matchArchiveFiles returned error: %v", err)
	}
	if len(got) != 1 || filepath.Base(got[0]) != "CBP-YYYY.md" {
		t.Errorf("matched %v, want [CBP-YYYY.md]", got)
	}

	got, err = matchArchiveFiles(dir, []string{"*.md"})
	if err != nil {
		t.Fatalf("matchArchiveFiles returned error: %v", err)
	}
	if len(got) != 1 || filepath.Base(got[0]) != "CBP-YYYY.md" {
		t.Errorf("glob matched %v, want [CBP-YYYY.md]", got)
	}
}

// TestMatchArchiveFiles_NoMatch verifies an unmatched ticket ID pattern
// returns no files instead of erroring.
func TestMatchArchiveFiles_NoMatch(t *testing.T) {
	dir := t.TempDir()

	got, err := matchArchiveFiles(dir, []string{"CBP-ZZZZ"})
	if err != nil {
		t.Fatalf("matchArchiveFiles returned error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("matched %v, want none", got)
	}
}