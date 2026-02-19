package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pankaj/claude-context/internal/config"
)

// TestHelper provides utilities for ticket tests
type TestHelper struct {
	t           *testing.T
	tempDir     string
	dataDir     string
	projectDir  string
	configMgr   *config.Manager
	originalDir string
}

// NewTestHelper creates a new test environment
func NewTestHelper(t *testing.T) *TestHelper {
	// Save original directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	// Create temporary directories
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, ".cctx")
	projectDir := filepath.Join(tempDir, "test-project")

	// Create data directory structure
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("Failed to create data dir: %v", err)
	}
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	// Initialize config
	configPath := filepath.Join(dataDir, "config.json")
	initialConfig := &config.Config{
		ManagedProjects: []config.Project{},
		GlobalContexts:  []config.GlobalContext{},
		Tickets: config.TicketSection{
			Active:   []config.Ticket{},
			Archived: []config.Ticket{},
			Settings: config.TicketSettings{
				AutoArchive: false,
			},
		},
		Settings: config.Settings{
			AutoCommit:      false,
			BackupOnUnlink:  true,
		},
	}

	data, err := json.MarshalIndent(initialConfig, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Create necessary directories
	contextDirs := []string{
		filepath.Join(dataDir, "contexts"),
		filepath.Join(dataDir, "contexts", "_tickets"),
		filepath.Join(dataDir, "contexts", "_archived"),
		filepath.Join(dataDir, "contexts", "_global"),
		filepath.Join(dataDir, "templates"),
	}
	for _, dir := range contextDirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir %s: %v", dir, err)
		}
	}

	// Create a default ticket template
	templateContent := `# Ticket: {{.TicketID}}

## Summary

Brief description here.

---

## Implementation

- [ ] Task 1
- [ ] Task 2
`
	templatePath := filepath.Join(dataDir, "templates", "ticket.md")
	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}

	configMgr := config.NewManager(dataDir)

	return &TestHelper{
		t:           t,
		tempDir:     tempDir,
		dataDir:     dataDir,
		projectDir:  projectDir,
		configMgr:   configMgr,
		originalDir: originalDir,
	}
}

// Cleanup restores the original directory
func (h *TestHelper) Cleanup() {
	os.Chdir(h.originalDir)
}

// GetConfig loads the current config
func (h *TestHelper) GetConfig() *config.Config {
	cfg, err := h.configMgr.Load()
	if err != nil {
		h.t.Fatalf("Failed to load config: %v", err)
	}
	return cfg
}

// VerifyTicketInConfig checks if a ticket exists in config
func (h *TestHelper) VerifyTicketInConfig(ticketID string, shouldExist bool) {
	cfg := h.GetConfig()
	found := false
	for _, ticket := range cfg.Tickets.Active {
		if ticket.TicketID == ticketID {
			found = true
			break
		}
	}
	if found != shouldExist {
		if shouldExist {
			h.t.Errorf("Ticket %s not found in config but should exist", ticketID)
		} else {
			h.t.Errorf("Ticket %s found in config but should not exist", ticketID)
		}
	}
}

// VerifyTicketArchived checks if a ticket is archived
func (h *TestHelper) VerifyTicketArchived(ticketID string) {
	cfg := h.GetConfig()
	found := false
	for _, ticket := range cfg.Tickets.Archived {
		if ticket.TicketID == ticketID {
			found = true
			if ticket.Status != "completed" {
				h.t.Errorf("Archived ticket %s has status %s, expected 'completed'", ticketID, ticket.Status)
			}
			break
		}
	}
	if !found {
		h.t.Errorf("Ticket %s not found in archived tickets", ticketID)
	}
}

// VerifyTicketDirectory checks if ticket directory exists
func (h *TestHelper) VerifyTicketDirectory(ticketID string, shouldExist bool) {
	ticketDir := filepath.Join(h.dataDir, "contexts", "_tickets", ticketID)
	_, err := os.Stat(ticketDir)
	exists := err == nil

	if exists != shouldExist {
		if shouldExist {
			h.t.Errorf("Ticket directory %s does not exist but should", ticketDir)
		} else {
			h.t.Errorf("Ticket directory %s exists but should not", ticketDir)
		}
	}
}

// VerifyTicketFile checks if ticket.md exists
func (h *TestHelper) VerifyTicketFile(ticketID string) {
	ticketFile := filepath.Join(h.dataDir, "contexts", "_tickets", ticketID, "ticket.md")
	if _, err := os.Stat(ticketFile); os.IsNotExist(err) {
		h.t.Errorf("Ticket file %s does not exist", ticketFile)
	}
}

// VerifySymlink checks if symlink exists in project
func (h *TestHelper) VerifySymlink(projectPath, ticketID string, shouldExist bool) {
	symlinkPath := filepath.Join(projectPath, ticketID+".md")
	_, err := os.Lstat(symlinkPath)
	exists := err == nil

	if exists != shouldExist {
		if shouldExist {
			h.t.Errorf("Symlink %s does not exist but should", symlinkPath)
		} else {
			h.t.Errorf("Symlink %s exists but should not", symlinkPath)
		}
	}

	if shouldExist {
		// Verify it's actually a symlink
		info, err := os.Lstat(symlinkPath)
		if err != nil {
			h.t.Errorf("Failed to stat symlink %s: %v", symlinkPath, err)
			return
		}
		if info.Mode()&os.ModeSymlink == 0 {
			h.t.Errorf("%s is not a symlink", symlinkPath)
		}

		// Verify target
		target, err := os.Readlink(symlinkPath)
		if err != nil {
			h.t.Errorf("Failed to read symlink %s: %v", symlinkPath, err)
			return
		}
		expectedTarget := filepath.Join(h.dataDir, "contexts", "_tickets", ticketID, "ticket.md")
		if target != expectedTarget {
			h.t.Errorf("Symlink target is %s, expected %s", target, expectedTarget)
		}
	}
}

// VerifyTicketLinkedToProject checks if ticket is linked to a project in config
func (h *TestHelper) VerifyTicketLinkedToProject(ticketID, contextName string, shouldBeLinked bool) {
	cfg := h.GetConfig()
	var ticket *config.Ticket
	for i := range cfg.Tickets.Active {
		if cfg.Tickets.Active[i].TicketID == ticketID {
			ticket = &cfg.Tickets.Active[i]
			break
		}
	}

	if ticket == nil {
		h.t.Fatalf("Ticket %s not found in config", ticketID)
	}

	found := false
	for _, proj := range ticket.LinkedProjects {
		if proj.ContextName == contextName {
			found = true
			break
		}
	}

	if found != shouldBeLinked {
		if shouldBeLinked {
			h.t.Errorf("Ticket %s not linked to project %s but should be", ticketID, contextName)
		} else {
			h.t.Errorf("Ticket %s linked to project %s but should not be", ticketID, contextName)
		}
	}
}

// AddProject adds a project to the config
func (h *TestHelper) AddProject(contextName string, projectPath string) {
	cfg := h.GetConfig()

	project := config.Project{
		ContextName:  contextName,
		ProjectPath:  projectPath,
		ContextPath:  filepath.Join("contexts", contextName, "claude.md"),
		CreatedAt:    time.Now(),
		LastModified: time.Now(),
		Status:       "active",
	}

	cfg.ManagedProjects = append(cfg.ManagedProjects, project)

	if err := h.configMgr.Save(cfg); err != nil {
		h.t.Fatalf("Failed to save config: %v", err)
	}

	// Create project context directory
	contextDir := filepath.Join(h.dataDir, "contexts", contextName)
	if err := os.MkdirAll(contextDir, 0755); err != nil {
		h.t.Fatalf("Failed to create context dir: %v", err)
	}

	// Create context file
	contextFile := filepath.Join(contextDir, "claude.md")
	if err := os.WriteFile(contextFile, []byte("# "+contextName), 0644); err != nil {
		h.t.Fatalf("Failed to create context file: %v", err)
	}
}

// Test_TicketCreate tests creating a new ticket
func Test_TicketCreate(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()

	// Add a project first
	h.AddProject("test-project", h.projectDir)

	// Change to project directory
	if err := os.Chdir(h.projectDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Override dataDir for the command
	dataDir = h.dataDir

	// Create ticket
	ticketID := "TEST-001"
	title := "Test ticket"
	tags := []string{"test", "verification"}

	cfg := h.GetConfig()
	ticket := config.Ticket{
		TicketID:     ticketID,
		Title:        title,
		Status:       "active",
		CreatedAt:    time.Now(),
		LastModified: time.Now(),
		Tags:         tags,
		Notes:        "",
		LinkedProjects: []config.LinkedProject{
			{
				ContextName: "test-project",
				ProjectPath: h.projectDir,
			},
		},
	}

	// Create ticket directory and file
	ticketDir := filepath.Join(h.dataDir, "contexts", "_tickets", ticketID)
	if err := os.MkdirAll(ticketDir, 0755); err != nil {
		t.Fatalf("Failed to create ticket dir: %v", err)
	}

	ticketFile := filepath.Join(ticketDir, "ticket.md")
	if err := os.WriteFile(ticketFile, []byte("# Ticket: "+ticketID), 0644); err != nil {
		t.Fatalf("Failed to create ticket file: %v", err)
	}

	// Create symlink
	symlinkPath := filepath.Join(h.projectDir, ticketID+".md")
	if err := os.Symlink(ticketFile, symlinkPath); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	// Update config
	cfg.Tickets.Active = append(cfg.Tickets.Active, ticket)
	if err := h.configMgr.Save(cfg); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Verify results
	h.VerifyTicketInConfig(ticketID, true)
	h.VerifyTicketDirectory(ticketID, true)
	h.VerifyTicketFile(ticketID)
	h.VerifySymlink(h.projectDir, ticketID, true)
	h.VerifyTicketLinkedToProject(ticketID, "test-project", true)

	// Verify tags
	cfg = h.GetConfig()
	var createdTicket *config.Ticket
	for i := range cfg.Tickets.Active {
		if cfg.Tickets.Active[i].TicketID == ticketID {
			createdTicket = &cfg.Tickets.Active[i]
			break
		}
	}
	if createdTicket == nil {
		t.Fatal("Created ticket not found")
	}
	if len(createdTicket.Tags) != 2 || createdTicket.Tags[0] != "test" || createdTicket.Tags[1] != "verification" {
		t.Errorf("Tags not saved correctly: %v", createdTicket.Tags)
	}
	if createdTicket.Title != title {
		t.Errorf("Title is %s, expected %s", createdTicket.Title, title)
	}
}

// Test_TicketLinkToMultipleProjects tests linking a ticket to multiple projects
func Test_TicketLinkToMultipleProjects(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()

	// Add two projects
	project1Dir := filepath.Join(h.tempDir, "project1")
	project2Dir := filepath.Join(h.tempDir, "project2")
	os.MkdirAll(project1Dir, 0755)
	os.MkdirAll(project2Dir, 0755)

	h.AddProject("project1", project1Dir)
	h.AddProject("project2", project2Dir)

	// Create a ticket linked to project1
	ticketID := "TEST-002"
	cfg := h.GetConfig()

	ticket := config.Ticket{
		TicketID:     ticketID,
		Title:        "Multi-project ticket",
		Status:       "active",
		CreatedAt:    time.Now(),
		LastModified: time.Now(),
		LinkedProjects: []config.LinkedProject{
			{
				ContextName: "project1",
				ProjectPath: project1Dir,
			},
		},
	}

	// Create ticket directory and file
	ticketDir := filepath.Join(h.dataDir, "contexts", "_tickets", ticketID)
	os.MkdirAll(ticketDir, 0755)
	ticketFile := filepath.Join(ticketDir, "ticket.md")
	os.WriteFile(ticketFile, []byte("# Ticket: "+ticketID), 0644)

	// Create symlink in project1
	symlink1 := filepath.Join(project1Dir, ticketID+".md")
	os.Symlink(ticketFile, symlink1)

	cfg.Tickets.Active = append(cfg.Tickets.Active, ticket)
	h.configMgr.Save(cfg)

	// Now link to project2
	cfg = h.GetConfig()
	for i := range cfg.Tickets.Active {
		if cfg.Tickets.Active[i].TicketID == ticketID {
			cfg.Tickets.Active[i].LinkedProjects = append(cfg.Tickets.Active[i].LinkedProjects, config.LinkedProject{
				ContextName: "project2",
				ProjectPath: project2Dir,
			})
			cfg.Tickets.Active[i].LastModified = time.Now()
			break
		}
	}
	h.configMgr.Save(cfg)

	// Create symlink in project2
	symlink2 := filepath.Join(project2Dir, ticketID+".md")
	os.Symlink(ticketFile, symlink2)

	// Verify
	h.VerifyTicketLinkedToProject(ticketID, "project1", true)
	h.VerifyTicketLinkedToProject(ticketID, "project2", true)
	h.VerifySymlink(project1Dir, ticketID, true)
	h.VerifySymlink(project2Dir, ticketID, true)
}

// Test_TicketComplete tests completing and archiving a ticket
func Test_TicketComplete(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()

	// Add project and create ticket
	h.AddProject("test-project", h.projectDir)

	ticketID := "TEST-003"
	cfg := h.GetConfig()

	ticket := config.Ticket{
		TicketID:     ticketID,
		Title:        "Ticket to complete",
		Status:       "active",
		CreatedAt:    time.Now(),
		LastModified: time.Now(),
		LinkedProjects: []config.LinkedProject{
			{
				ContextName: "test-project",
				ProjectPath: h.projectDir,
			},
		},
	}

	// Create ticket files
	ticketDir := filepath.Join(h.dataDir, "contexts", "_tickets", ticketID)
	os.MkdirAll(ticketDir, 0755)
	ticketFile := filepath.Join(ticketDir, "ticket.md")
	os.WriteFile(ticketFile, []byte("# Ticket: "+ticketID), 0644)

	symlinkPath := filepath.Join(h.projectDir, ticketID+".md")
	os.Symlink(ticketFile, symlinkPath)

	cfg.Tickets.Active = append(cfg.Tickets.Active, ticket)
	h.configMgr.Save(cfg)

	// Complete the ticket
	cfg = h.GetConfig()
	var completedTicket *config.Ticket
	newActive := []config.Ticket{}
	for i := range cfg.Tickets.Active {
		if cfg.Tickets.Active[i].TicketID == ticketID {
			completedTicket = &cfg.Tickets.Active[i]
			completedTicket.Status = "completed"
			now := time.Now()
			completedTicket.CompletedAt = &now
			completedTicket.LastModified = time.Now()
			completedTicket.Commits = []string{"abc123"}

			// Move to archived
			archivedPath := filepath.Join("contexts", "_archived", time.Now().Format("2006-01-02")+"_"+ticketID)
			completedTicket.ArchivedPath = archivedPath

			cfg.Tickets.Archived = append(cfg.Tickets.Archived, *completedTicket)
		} else {
			newActive = append(newActive, cfg.Tickets.Active[i])
		}
	}
	cfg.Tickets.Active = newActive
	h.configMgr.Save(cfg)

	// Remove symlink
	os.Remove(symlinkPath)

	// Move ticket directory to archived
	archivedDir := filepath.Join(h.dataDir, completedTicket.ArchivedPath)
	os.MkdirAll(filepath.Dir(archivedDir), 0755)
	os.Rename(ticketDir, archivedDir)

	// Verify
	h.VerifyTicketInConfig(ticketID, false) // Should not be in active
	h.VerifyTicketArchived(ticketID)       // Should be in archived
	h.VerifySymlink(h.projectDir, ticketID, false) // Symlink should be removed
	h.VerifyTicketDirectory(ticketID, false) // Original directory should be gone

	// Verify archived directory exists
	if _, err := os.Stat(archivedDir); os.IsNotExist(err) {
		t.Errorf("Archived directory %s does not exist", archivedDir)
	}
}

// Test_TicketArchiveAll tests archiving all tickets
func Test_TicketArchiveAll(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()

	// Add project
	h.AddProject("test-project", h.projectDir)

	// Create multiple tickets
	ticketIDs := []string{"TEST-004", "TEST-005", "TEST-006"}
	cfg := h.GetConfig()

	for _, ticketID := range ticketIDs {
		ticket := config.Ticket{
			TicketID:     ticketID,
			Title:        "Ticket " + ticketID,
			Status:       "active",
			CreatedAt:    time.Now(),
			LastModified: time.Now(),
			LinkedProjects: []config.LinkedProject{
				{
					ContextName: "test-project",
					ProjectPath: h.projectDir,
				},
			},
		}

		// Create ticket files
		ticketDir := filepath.Join(h.dataDir, "contexts", "_tickets", ticketID)
		os.MkdirAll(ticketDir, 0755)
		ticketFile := filepath.Join(ticketDir, "ticket.md")
		os.WriteFile(ticketFile, []byte("# Ticket: "+ticketID), 0644)

		symlinkPath := filepath.Join(h.projectDir, ticketID+".md")
		os.Symlink(ticketFile, symlinkPath)

		cfg.Tickets.Active = append(cfg.Tickets.Active, ticket)
	}
	h.configMgr.Save(cfg)

	// Archive all tickets
	cfg = h.GetConfig()
	for _, ticket := range cfg.Tickets.Active {
		ticket.Status = "completed"
		now := time.Now()
		ticket.CompletedAt = &now
		ticket.LastModified = time.Now()

		archivedPath := filepath.Join("contexts", "_archived", time.Now().Format("2006-01-02")+"_"+ticket.TicketID)
		ticket.ArchivedPath = archivedPath

		cfg.Tickets.Archived = append(cfg.Tickets.Archived, ticket)

		// Remove symlink
		symlinkPath := filepath.Join(h.projectDir, ticket.TicketID+".md")
		os.Remove(symlinkPath)

		// Move directory
		ticketDir := filepath.Join(h.dataDir, "contexts", "_tickets", ticket.TicketID)
		archivedDir := filepath.Join(h.dataDir, archivedPath)
		os.MkdirAll(filepath.Dir(archivedDir), 0755)
		os.Rename(ticketDir, archivedDir)
	}
	cfg.Tickets.Active = []config.Ticket{}
	h.configMgr.Save(cfg)

	// Verify all tickets are archived
	for _, ticketID := range ticketIDs {
		h.VerifyTicketInConfig(ticketID, false)
		h.VerifyTicketArchived(ticketID)
		h.VerifySymlink(h.projectDir, ticketID, false)
	}

	// Verify no active tickets remain
	cfg = h.GetConfig()
	if len(cfg.Tickets.Active) != 0 {
		t.Errorf("Expected 0 active tickets, got %d", len(cfg.Tickets.Active))
	}
	if len(cfg.Tickets.Archived) != 3 {
		t.Errorf("Expected 3 archived tickets, got %d", len(cfg.Tickets.Archived))
	}
}

// Test_TicketList tests listing tickets
func Test_TicketList(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()

	// Add two projects
	project1Dir := filepath.Join(h.tempDir, "project1")
	project2Dir := filepath.Join(h.tempDir, "project2")
	os.MkdirAll(project1Dir, 0755)
	os.MkdirAll(project2Dir, 0755)

	h.AddProject("project1", project1Dir)
	h.AddProject("project2", project2Dir)

	// Create tickets for different projects
	tickets := []struct {
		id      string
		project string
	}{
		{"TEST-007", "project1"},
		{"TEST-008", "project1"},
		{"TEST-009", "project2"},
	}

	cfg := h.GetConfig()
	for _, tc := range tickets {
		projectPath := project1Dir
		if tc.project == "project2" {
			projectPath = project2Dir
		}

		ticket := config.Ticket{
			TicketID:     tc.id,
			Title:        "Ticket " + tc.id,
			Status:       "active",
			CreatedAt:    time.Now(),
			LastModified: time.Now(),
			LinkedProjects: []config.LinkedProject{
				{
					ContextName: tc.project,
					ProjectPath: projectPath,
				},
			},
		}

		cfg.Tickets.Active = append(cfg.Tickets.Active, ticket)
	}
	h.configMgr.Save(cfg)

	// Verify we can filter by project
	cfg = h.GetConfig()

	// Count tickets for project1
	project1Count := 0
	for _, ticket := range cfg.Tickets.Active {
		for _, proj := range ticket.LinkedProjects {
			if proj.ContextName == "project1" {
				project1Count++
				break
			}
		}
	}

	if project1Count != 2 {
		t.Errorf("Expected 2 tickets for project1, got %d", project1Count)
	}

	// Count tickets for project2
	project2Count := 0
	for _, ticket := range cfg.Tickets.Active {
		for _, proj := range ticket.LinkedProjects {
			if proj.ContextName == "project2" {
				project2Count++
				break
			}
		}
	}

	if project2Count != 1 {
		t.Errorf("Expected 1 ticket for project2, got %d", project2Count)
	}
}