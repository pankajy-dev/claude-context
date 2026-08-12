package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsTicketFileName(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		ticketID string
		want     bool
	}{
		{"ticket file itself", "CBP-52228.md", "CBP-52228", true},
		{"plan prefix", "PLAN-CBP-52228.md", "CBP-52228", true},
		{"notes suffix", "CBP-52228-notes.md", "CBP-52228", true},
		{"underscore separator", "CBP-52228_design.md", "CBP-52228", true},
		{"embedded in middle", "review-CBP-52228-v2.md", "CBP-52228", true},
		{"non-md extension", "CBP-52228.txt", "CBP-52228", true},
		{"patch file", "CBP-52228.patch", "CBP-52228", true},
		{"no extension", "CBP-52228", "CBP-52228", true},
		{"double extension", "CBP-52228.tar.gz", "CBP-52228", true},
		{"case insensitive", "plan-cbp-52228.md", "CBP-52228", true},

		{"longer id is not a match", "CBP-522280.md", "CBP-52228", false},
		{"digit glued before", "XCBP-52228.md", "CBP-52228", false},
		{"different ticket", "CBP-11111.md", "CBP-52228", false},
		{"unrelated doc", "platform-understanding.md", "CBP-52228", false},
		{"empty ticket id", "CBP-52228.md", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTicketFileName(tt.filename, tt.ticketID); got != tt.want {
				t.Errorf("IsTicketFileName(%q, %q) = %v, want %v", tt.filename, tt.ticketID, got, tt.want)
			}
		})
	}
}

func TestFindTicketFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"CBP-52228.md",
		"PLAN-CBP-52228.md",
		"CBP-52228-notes.txt",
		"platform-understanding.md",
		"unify-ci-runs-api.md",
		"CBP-11111.md",
		"tilt_config.json",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "CBP-52228-dir"), 0755); err != nil {
		t.Fatal(err)
	}

	files := FindTicketFiles(dir, "CBP-52228")
	if len(files) != 3 {
		t.Fatalf("expected 3 ticket files, got %d: %v", len(files), files)
	}
	if base := filepath.Base(files[0]); base != "CBP-52228.md" {
		t.Errorf("expected canonical ticket file first, got %q", base)
	}

	extras := FindExtraTicketFiles(dir, "CBP-52228")
	if len(extras) != 2 {
		t.Fatalf("expected 2 companions, got %d: %v", len(extras), extras)
	}
	for _, e := range extras {
		if e == "CBP-52228.md" {
			t.Error("companion list must exclude the canonical ticket file")
		}
	}
}

func TestFindExtraTicketFilesSkipsSessions(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"CBP-1.md", "SESSIONS.md", "PLAN-CBP-1.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	extras := FindExtraTicketFiles(dir, "CBP-1")
	if len(extras) != 1 || extras[0] != "PLAN-CBP-1.md" {
		t.Errorf("expected only PLAN-CBP-1.md, got %v", extras)
	}
}
