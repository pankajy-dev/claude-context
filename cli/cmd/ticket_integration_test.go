package cmd

import (
	"os"
	"path/filepath"
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

	// Verify ticket was created
	h.VerifyTicketInConfig(ticketID, true)
	h.VerifyTicketDirectory(ticketID, true)
	h.VerifyTicketFile(ticketID)

	// THIS IS THE KEY TEST: Verify auto-detection worked
	// The ticket should be automatically linked to "test-project"
	h.VerifyTicketLinkedToProject(ticketID, "test-project", true)

	// Verify symlink was created in the project directory
	h.VerifySymlink(h.projectDir, ticketID, true)

	// Verify the ticket has the correct data
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

	if ticket.Title != ticketTitle {
		t.Errorf("Title is %s, expected %s", ticket.Title, ticketTitle)
	}

	if len(ticket.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(ticket.Tags))
	}

	if len(ticket.LinkedProjects) == 0 {
		t.Fatal("Expected ticket to be auto-linked to project, but LinkedProjects is empty")
	}

	if ticket.LinkedProjects[0].ContextName != "test-project" {
		t.Errorf("Expected auto-link to 'test-project', got %s", ticket.LinkedProjects[0].ContextName)
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

	// Stay in temp directory (not a managed project)
	if err := os.Chdir(h.tempDir); err != nil {
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

	// Create ticket file
	ticketDir := filepath.Join(h.dataDir, "contexts", "_tickets", ticketID)
	os.MkdirAll(ticketDir, 0755)
	ticketFile := filepath.Join(ticketDir, "ticket.md")
	os.WriteFile(ticketFile, []byte("# Ticket: "+ticketID), 0644)

	// Now link it to project1 using the actual command
	ticketFlag = ticketID
	cmd := &cobra.Command{}
	args := []string{"project1"}

	err := runTicketLink(cmd, args)
	if err != nil {
		t.Fatalf("runTicketLink failed: %v", err)
	}

	// Verify ticket is now linked to project1
	h.VerifyTicketLinkedToProject(ticketID, "project1", true)

	// Verify symlink was created
	h.VerifySymlink(project1Dir, ticketID, true)

	// Link to project2 as well
	args = []string{"project2"}
	err = runTicketLink(cmd, args)
	if err != nil {
		t.Fatalf("runTicketLink to project2 failed: %v", err)
	}

	// Verify ticket is linked to both projects
	h.VerifyTicketLinkedToProject(ticketID, "project1", true)
	h.VerifyTicketLinkedToProject(ticketID, "project2", true)
	h.VerifySymlink(project1Dir, ticketID, true)
	h.VerifySymlink(project2Dir, ticketID, true)

	// Reset globals
	ticketFlag = ""
}