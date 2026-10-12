package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pankaj/claude-context/internal/config"
	"github.com/spf13/cobra"
)

// Test_TicketCreateIntegration tests the actual ticket create command with auto-detection
func Test_TicketCreateIntegration(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()

	// Add a project
	h.AddProject("test-project", h.projectDir)

	// Change to project directory (this is where auto-detection should work)
	if err := os.Chdir(h.projectDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Override dataDir global variable
	dataDir = h.dataDir

	// Debug: Verify paths
	currentDir, _ := os.Getwd()
	t.Logf("Current dir: %s", currentDir)
	t.Logf("Project dir: %s", h.projectDir)

	debugCfg := h.GetConfig()
	for _, proj := range debugCfg.ManagedProjects {
		t.Logf("Managed project: %s at %s", proj.ContextName, proj.ProjectPath)
	}

	// Set ticket parameters
	ticketID := "INTEGRATION-001"
	ticketTitle = "Integration test ticket"
	ticketTags = "integration,test"
	ticketNotes = "Testing auto-detection"

	// Create a mock command with args
	cmd := &cobra.Command{}
	args := []string{ticketID}

	// Call the actual command function
	err := runTicketCreate(cmd, args)
	if err != nil {
		t.Fatalf("runTicketCreate failed: %v", err)
	}

	// Verify ticket was created as a .cctx/<ID>/ workspace
	h.VerifyTicketInConfig(ticketID, true)
	workspace := filepath.Join(h.projectDir, ".cctx", ticketID)
	for _, name := range []string{"requirements.md", "plan.md", "validation.md", "changelog.md"} {
		content, err := os.ReadFile(filepath.Join(workspace, name))
		if err != nil {
			t.Errorf("missing %s: %v", name, err)
			continue
		}
		if strings.Contains(string(content), "{{TICKET_ID}}") || !strings.Contains(string(content), ticketID) {
			t.Errorf("%s: ticket ID not substituted", name)
		}
	}
	if _, err := os.Lstat(filepath.Join(h.projectDir, ticketID+".md")); err == nil {
		t.Error("no <ID>.md should be created")
	}
	h.VerifyTicketDirectory(ticketID, false)

	cfg := h.GetConfig()
	ticket := cfg.GetTicket(ticketID, false)
	if ticket == nil {
		t.Fatal("Ticket not found in config")
	}
	if ticket.WorkspacePath != canon(workspace) {
		t.Errorf("workspace_path = %s, want %s", ticket.WorkspacePath, canon(workspace))
	}
	if len(ticket.LinkedProjects) != 0 || ticket.PrimaryContextName != "" {
		t.Errorf("directory tickets have no linked projects: %+v", ticket)
	}
	if ticket.Title != ticketTitle {
		t.Errorf("Title is %s, expected %s", ticket.Title, ticketTitle)
	}
	if len(ticket.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(ticket.Tags))
	}

	// Reset globals
	ticketTitle = ""
	ticketTags = ""
	ticketNotes = ""
}

// Test_TicketCreateIntegration_NoProject tests ticket creation when NOT in a project directory
func Test_TicketCreateIntegration_NoProject(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()

	// Add a project, but DON'T change to its directory
	h.AddProject("test-project", h.projectDir)

	// Work from a directory that isn't a managed project
	unmanaged := filepath.Join(h.tempDir, "unmanaged")
	os.MkdirAll(unmanaged, 0755)
	if err := os.Chdir(unmanaged); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Override dataDir global variable
	dataDir = h.dataDir

	// Set ticket parameters
	ticketID := "INTEGRATION-002"
	ticketTitle = "No auto-link test"

	// Call the actual command function
	cmd := &cobra.Command{}
	args := []string{ticketID}

	err := runTicketCreate(cmd, args)
	if err != nil {
		t.Fatalf("runTicketCreate failed: %v", err)
	}

	// Verify ticket was created
	h.VerifyTicketInConfig(ticketID, true)

	// Verify ticket is NOT auto-linked (we're not in a project directory)
	cfg := h.GetConfig()
	var ticket *config.Ticket
	for i := range cfg.Tickets.Active {
		if cfg.Tickets.Active[i].TicketID == ticketID {
			ticket = &cfg.Tickets.Active[i]
			break
		}
	}

	if ticket == nil {
		t.Fatal("Ticket not found in config")
	}

	if len(ticket.LinkedProjects) != 0 {
		t.Errorf("Expected ticket to NOT be auto-linked (not in project dir), but got %d linked projects", len(ticket.LinkedProjects))
	}

	// Reset globals
	ticketTitle = ""
}

// Test_TicketLinkIntegration tests the actual ticket link command
func Test_TicketLinkIntegration(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()

	// Add two projects
	project1Dir := filepath.Join(h.tempDir, "project1")
	project2Dir := filepath.Join(h.tempDir, "project2")
	os.MkdirAll(project1Dir, 0755)
	os.MkdirAll(project2Dir, 0755)

	h.AddProject("project1", project1Dir)
	h.AddProject("project2", project2Dir)

	// Create a ticket manually (not linked to anything)
	ticketID := "INTEGRATION-003"
	dataDir = h.dataDir

	cfg := h.GetConfig()
	ticket := config.Ticket{
		TicketID:       ticketID,
		Title:          "Link test",
		Status:         "active",
		CreatedAt:      time.Now(),
		LastModified:   time.Now(),
		LinkedProjects: []config.LinkedProject{}, // Empty - not linked yet
	}
	cfg.Tickets.Active = append(cfg.Tickets.Active, ticket)
	h.configMgr.Save(cfg)

	// Create ticket file in the data dir under its canonical name
	ticketDir := filepath.Join(h.dataDir, "contexts", "_tickets", ticketID)
	os.MkdirAll(ticketDir, 0755)
	ticketFile := filepath.Join(ticketDir, ticketID+".md")
	os.WriteFile(ticketFile, []byte("# Ticket: "+ticketID), 0644)

	// Now link it to project1 using the actual command:
	// ticket link <ticket-id> [project...]
	cmd := &cobra.Command{}
	args := []string{ticketID, "project1"}

	err := runTicketLink(cmd, args)
	if err != nil {
		t.Fatalf("runTicketLink failed: %v", err)
	}

	// Verify ticket is now linked to project1
	h.VerifyTicketLinkedToProject(ticketID, "project1", true)

	// Verify symlink was created
	h.VerifySymlink(project1Dir, ticketID, true)

	// Link to project2 as well
	args = []string{ticketID, "project2"}
	err = runTicketLink(cmd, args)
	if err != nil {
		t.Fatalf("runTicketLink to project2 failed: %v", err)
	}

	// Verify ticket is linked to both projects
	h.VerifyTicketLinkedToProject(ticketID, "project1", true)
	h.VerifyTicketLinkedToProject(ticketID, "project2", true)
	h.VerifySymlink(project1Dir, ticketID, true)
	h.VerifySymlink(project2Dir, ticketID, true)
}

// Test_TicketLinkMissingFileIntegration verifies that linking a ticket whose
// file is missing fails loudly instead of recording a link.
func Test_TicketLinkMissingFileIntegration(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()

	projectDir := filepath.Join(h.tempDir, "project1")
	os.MkdirAll(projectDir, 0755)
	h.AddProject("project1", projectDir)

	ticketID := "INTEGRATION-004"
	dataDir = h.dataDir

	cfg := h.GetConfig()
	cfg.Tickets.Active = append(cfg.Tickets.Active, config.Ticket{
		TicketID:       ticketID,
		Title:          "Missing file test",
		Status:         "active",
		CreatedAt:      time.Now(),
		LastModified:   time.Now(),
		LinkedProjects: []config.LinkedProject{},
	})
	h.configMgr.Save(cfg)

	// Deliberately create no ticket file, in the project or the data dir.
	if err := runTicketLink(&cobra.Command{}, []string{ticketID, "project1"}); err != nil {
		t.Fatalf("runTicketLink returned an unexpected error: %v", err)
	}

	h.VerifyTicketLinkedToProject(ticketID, "project1", false)

	after := h.GetConfig()
	for _, tk := range after.Tickets.Active {
		if tk.TicketID == ticketID && tk.PrimaryContextName != "" {
			t.Errorf("primary context was set to %q despite no ticket file", tk.PrimaryContextName)
		}
	}
}

// Test_TicketRestoreIntegration verifies that restoring an archived ticket
// copies its files back to the current directory, recreates the data dir
// symlinks, and moves the ticket back to active in config.json.
func Test_TicketRestoreIntegration(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()

	h.AddProject("project1", h.projectDir)

	ticketID := "INTEGRATION-005"
	archivedDirName := "2026-01-01_" + ticketID
	archivedDir := filepath.Join(h.dataDir, "contexts", "_archived", archivedDirName)
	if err := os.MkdirAll(archivedDir, 0755); err != nil {
		t.Fatalf("Failed to create archived dir: %v", err)
	}

	os.WriteFile(filepath.Join(archivedDir, ticketID+".md"), []byte("# Ticket: "+ticketID), 0644)
	os.WriteFile(filepath.Join(archivedDir, "SESSIONS.md"), []byte("# Sessions"), 0644)
	os.WriteFile(filepath.Join(archivedDir, "PLAN-"+ticketID+".md"), []byte("# Plan"), 0644)

	now := time.Now()
	cfg := h.GetConfig()
	cfg.Tickets.Archived = append(cfg.Tickets.Archived, config.Ticket{
		TicketID:           ticketID,
		Title:              "Restore test",
		Status:             "completed",
		CreatedAt:          now,
		LastModified:       now,
		CompletedAt:        &now,
		PrimaryContextName: "project1",
		ArchivedPath:       filepath.Join("contexts", "_archived", archivedDirName),
	})
	h.configMgr.Save(cfg)

	if err := os.Chdir(h.projectDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	dataDir = h.dataDir

	rootCmd.SetArgs([]string{"ticket", "restore", ticketID, "--force", "--data-dir", h.dataDir})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Failed to restore ticket: %v", err)
	}

	for _, name := range []string{ticketID + ".md", "SESSIONS.md", "PLAN-" + ticketID + ".md"} {
		if _, err := os.Stat(filepath.Join(h.projectDir, name)); err != nil {
			t.Errorf("Expected restored file %s in project dir: %v", name, err)
		}
	}

	if _, err := os.Stat(archivedDir); !os.IsNotExist(err) {
		t.Errorf("Expected archived dir %s to be removed", archivedDir)
	}

	h.VerifyTicketFile(ticketID)
	h.VerifySymlink(h.projectDir, ticketID, true)

	cfg = h.GetConfig()
	h.VerifyTicketInConfig(ticketID, true)
	for _, tk := range cfg.Tickets.Archived {
		if tk.TicketID == ticketID {
			t.Errorf("Ticket %s still present in archived tickets", ticketID)
		}
	}

	var restored *config.Ticket
	for i := range cfg.Tickets.Active {
		if cfg.Tickets.Active[i].TicketID == ticketID {
			restored = &cfg.Tickets.Active[i]
		}
	}
	if restored == nil {
		t.Fatalf("Restored ticket not found in active tickets")
	}
	if restored.Status != "active" {
		t.Errorf("Restored ticket status is %q, expected active", restored.Status)
	}
	if restored.ArchivedPath != "" {
		t.Errorf("Restored ticket still has archived_path: %s", restored.ArchivedPath)
	}
	if restored.CompletedAt != nil {
		t.Errorf("Restored ticket still has completed_at set")
	}
}

// Test_TicketDeleteRemovesFromWorkingList verifies that deleting a ticket also
// drops it from current_working_tickets, so it doesn't linger there forever.
func Test_TicketDeleteRemovesFromWorkingList(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()

	h.AddProject("project1", h.projectDir)

	ticketID := "INTEGRATION-006"
	now := time.Now()
	cfg := h.GetConfig()
	cfg.Tickets.Active = append(cfg.Tickets.Active, config.Ticket{
		TicketID:     ticketID,
		Status:       "active",
		CreatedAt:    now,
		LastModified: now,
	})
	cfg.AddCurrentWorkingTicket(ticketID, "project1")
	h.configMgr.Save(cfg)

	if !h.GetConfig().IsCurrentlyWorking(ticketID) {
		t.Fatalf("Setup failed: ticket not in working list before delete")
	}

	if err := os.Chdir(h.projectDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	dataDir = h.dataDir

	rootCmd.SetArgs([]string{"ticket", "delete", ticketID, "--force", "--data-dir", h.dataDir})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Failed to delete ticket: %v", err)
	}

	if h.GetConfig().IsCurrentlyWorking(ticketID) {
		t.Errorf("Ticket %s still in current_working_tickets after delete", ticketID)
	}
}

// Test_TicketCompleteRemovesStaleWorkingEntry covers a working-list entry whose
// ticket is already archived: it used to fail with "ticket not found" forever.
func Test_TicketCompleteRemovesStaleWorkingEntry(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()

	ticketID := "INTEGRATION-007"
	now := time.Now()
	cfg := h.GetConfig()
	cfg.Tickets.Archived = append(cfg.Tickets.Archived, config.Ticket{
		TicketID:     ticketID,
		Status:       "completed",
		CreatedAt:    now,
		LastModified: now,
		CompletedAt:  &now,
	})
	cfg.AddCurrentWorkingTicket(ticketID, "")
	h.configMgr.Save(cfg)

	if err := os.Chdir(h.tempDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	dataDir = h.dataDir

	if err := runTicketComplete(ticketCompleteCmd, []string{ticketID}); err != nil {
		t.Fatalf("runTicketComplete failed on stale working entry: %v", err)
	}

	if h.GetConfig().IsCurrentlyWorking(ticketID) {
		t.Errorf("Ticket %s still in current_working_tickets after complete", ticketID)
	}
}

// Test_TicketCompleteUnknownTicketStillErrors keeps the error for a ticket that
// exists nowhere, so typos are not silently swallowed.
func Test_TicketCompleteUnknownTicketStillErrors(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()

	if err := os.Chdir(h.tempDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	dataDir = h.dataDir

	if err := runTicketComplete(ticketCompleteCmd, []string{"NOPE-999"}); err == nil {
		t.Error("Expected error for unknown ticket, got nil")
	}
}

// Test_TicketCreatePrunesStaleWorkingEntries verifies stale entries are dropped
// before the prompt, so creating a new ticket no longer asks about them.
func Test_TicketCreatePrunesStaleWorkingEntries(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()

	h.AddProject("test-project", h.projectDir)

	staleID := "INTEGRATION-008-STALE"
	now := time.Now()
	cfg := h.GetConfig()
	cfg.Tickets.Archived = append(cfg.Tickets.Archived, config.Ticket{
		TicketID:     staleID,
		Status:       "completed",
		CreatedAt:    now,
		LastModified: now,
		CompletedAt:  &now,
	})
	cfg.AddCurrentWorkingTicket(staleID, "")
	h.configMgr.Save(cfg)

	if err := os.Chdir(h.projectDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	dataDir = h.dataDir

	newID := "INTEGRATION-008"
	if err := runTicketCreate(&cobra.Command{}, []string{newID}); err != nil {
		t.Fatalf("runTicketCreate failed: %v", err)
	}

	cfg = h.GetConfig()
	if cfg.IsCurrentlyWorking(staleID) {
		t.Errorf("Stale ticket %s still in current_working_tickets after create", staleID)
	}
	if !cfg.IsCurrentlyWorking(newID) {
		t.Errorf("New ticket %s not added to current_working_tickets", newID)
	}
}
