package cmd

import (
	"os"
	"path/filepath"
	"strings"
)

// Ticket file recognition lives here and nowhere else. A ticket's working set is
// its own file (CBP-1.md) plus any companion named after it, whatever the
// extension: PLAN-CBP-1.md, CBP-1-notes.txt, CBP-1.patch. Every command that
// moves, archives or cleans up ticket files must go through these helpers so
// they all agree on what belongs to a ticket.

// IsTicketFileName reports whether filename belongs to ticketID. The extension
// is ignored, but the ID must appear as a whole word so CBP-5 does not match
// CBP-52.md.
func IsTicketFileName(filename, ticketID string) bool {
	if ticketID == "" {
		return false
	}

	stem := strings.ToLower(strings.TrimSuffix(filename, filepath.Ext(filename)))
	id := strings.ToLower(ticketID)

	for from := 0; from <= len(stem)-len(id); {
		idx := strings.Index(stem[from:], id)
		if idx < 0 {
			return false
		}
		start := from + idx
		if !isWordByte(stem, start-1) && !isWordByte(stem, start+len(id)) {
			return true
		}
		from = start + 1
	}
	return false
}

func isWordByte(s string, i int) bool {
	if i < 0 || i >= len(s) {
		return false
	}
	c := s[i]
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// FindTicketFiles returns the full working set for a ticket in dir, ticket file
// first, then companions alphabetically. Paths are absolute.
func FindTicketFiles(dir, ticketID string) []string {
	names := ticketFileNames(dir, ticketID)
	files := make([]string, 0, len(names))
	for _, n := range names {
		files = append(files, filepath.Join(dir, n))
	}
	return files
}

// FindExtraTicketFiles returns only the companion docs in dir — the ticket's own
// file and SESSIONS.md are excluded. Names are relative to dir.
func FindExtraTicketFiles(dir, ticketID string) []string {
	extras := []string{}
	for _, n := range ticketFileNames(dir, ticketID) {
		if strings.EqualFold(n, ticketID+".md") || n == "SESSIONS.md" {
			continue
		}
		extras = append(extras, n)
	}
	return extras
}

// ticketFileNames lists matching file names in dir with the canonical ticket
// file sorted first.
func ticketFileNames(dir, ticketID string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	names := []string{}
	for _, e := range entries {
		if e.IsDir() || !IsTicketFileName(e.Name(), ticketID) {
			continue
		}
		names = append(names, e.Name())
	}
	sortStrings(names)

	canonical := ticketID + ".md"
	for i, n := range names {
		if strings.EqualFold(n, canonical) {
			names[0], names[i] = names[i], names[0]
			break
		}
	}
	return names
}
