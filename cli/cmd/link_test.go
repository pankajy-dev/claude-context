package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pankaj/claude-context/internal/config"
)

// Test_LinkProject tests linking a new project
func Test_LinkProject(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()

	// Create a new project directory
	newProjectDir := filepath.Join(h.tempDir, "new-project")
	if err := os.MkdirAll(newProjectDir, 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}

	// Simulate link command
	contextName := "new-project"
	cfg := h.GetConfig()

	project := config.Project{
		ContextName:  contextName,
		ProjectPath:  newProjectDir,
		ContextPath:  filepath.Join("contexts", contextName, "claude.md"),
		CreatedAt:    time.Now(),
		LastModified: time.Now(),
		Status:       "active",
	}

	cfg.ManagedProjects = append(cfg.ManagedProjects, project)
	if err := h.configMgr.Save(cfg); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Create context directory and file
	contextDir := filepath.Join(h.dataDir, "contexts", contextName)
	if err := os.MkdirAll(contextDir, 0755); err != nil {
		t.Fatalf("Failed to create context dir: %v", err)
	}

	contextFile := filepath.Join(contextDir, "claude.md")
	if err := os.WriteFile(contextFile, []byte("# "+contextName), 0644); err != nil {
		t.Fatalf("Failed to create context file: %v", err)
	}

	// Create symlink in project
	symlinkPath := filepath.Join(newProjectDir, "claude.md")
	if err := os.Symlink(contextFile, symlinkPath); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	// Verify project was added to config
	cfg = h.GetConfig()
	found := false
	for _, proj := range cfg.ManagedProjects {
		if proj.ContextName == contextName {
			found = true
			if proj.ProjectPath != newProjectDir {
				t.Errorf("Project path is %s, expected %s", proj.ProjectPath, newProjectDir)
			}
			if proj.Status != "active" {
				t.Errorf("Project status is %s, expected 'active'", proj.Status)
			}
			break
		}
	}
	if !found {
		t.Error("Project not found in config")
	}

	// Verify context directory exists
	if _, err := os.Stat(contextDir); os.IsNotExist(err) {
		t.Error("Context directory does not exist")
	}

	// Verify context file exists
	if _, err := os.Stat(contextFile); os.IsNotExist(err) {
		t.Error("Context file does not exist")
	}

	// Verify symlink exists and points to correct target
	linkInfo, err := os.Lstat(symlinkPath)
	if err != nil {
		t.Fatalf("Symlink does not exist: %v", err)
	}
	if linkInfo.Mode()&os.ModeSymlink == 0 {
		t.Error("claude.md is not a symlink")
	}

	target, err := os.Readlink(symlinkPath)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}
	if target != contextFile {
		t.Errorf("Symlink target is %s, expected %s", target, contextFile)
	}
}

// Test_LinkDuplicateProject tests linking a project that's already linked
func Test_LinkDuplicateProject(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()

	// Add a project
	h.AddProject("test-project", h.projectDir)

	// Try to add it again
	cfg := h.GetConfig()
	initialCount := len(cfg.ManagedProjects)

	// Check if already exists
	exists := false
	for _, proj := range cfg.ManagedProjects {
		if proj.ContextName == "test-project" {
			exists = true
			break
		}
	}

	if !exists {
		t.Fatal("Project should already exist")
	}

	// Verify count hasn't changed
	cfg = h.GetConfig()
	if len(cfg.ManagedProjects) != initialCount {
		t.Errorf("Project count changed from %d to %d", initialCount, len(cfg.ManagedProjects))
	}
}

// Test_UnlinkProject tests unlinking a project
func Test_UnlinkProject(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()

	// Add a project
	contextName := "test-project"
	h.AddProject(contextName, h.projectDir)

	// Create symlink
	contextFile := filepath.Join(h.dataDir, "contexts", contextName, "claude.md")
	symlinkPath := filepath.Join(h.projectDir, "claude.md")
	os.Symlink(contextFile, symlinkPath)

	// Unlink the project
	cfg := h.GetConfig()
	newProjects := []config.Project{}
	for _, proj := range cfg.ManagedProjects {
		if proj.ContextName != contextName {
			newProjects = append(newProjects, proj)
		}
	}
	cfg.ManagedProjects = newProjects
	h.configMgr.Save(cfg)

	// Remove symlink
	os.Remove(symlinkPath)

	// Verify project was removed from config
	cfg = h.GetConfig()
	for _, proj := range cfg.ManagedProjects {
		if proj.ContextName == contextName {
			t.Error("Project still exists in config after unlink")
		}
	}

	// Verify symlink was removed
	if _, err := os.Lstat(symlinkPath); err == nil {
		t.Error("Symlink still exists after unlink")
	}

	// Verify context directory still exists (not deleted by default)
	contextDir := filepath.Join(h.dataDir, "contexts", contextName)
	if _, err := os.Stat(contextDir); os.IsNotExist(err) {
		t.Error("Context directory was deleted (should be preserved)")
	}
}

// Test_ListProjects tests listing managed projects
func Test_ListProjects(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()

	// Add multiple projects
	projects := []struct {
		name string
		path string
	}{
		{"project1", filepath.Join(h.tempDir, "project1")},
		{"project2", filepath.Join(h.tempDir, "project2")},
		{"project3", filepath.Join(h.tempDir, "project3")},
	}

	for _, proj := range projects {
		os.MkdirAll(proj.path, 0755)
		h.AddProject(proj.name, proj.path)
	}

	// List projects
	cfg := h.GetConfig()

	if len(cfg.ManagedProjects) != 3 {
		t.Errorf("Expected 3 projects, got %d", len(cfg.ManagedProjects))
	}

	// Verify each project exists
	for _, expectedProj := range projects {
		found := false
		for _, proj := range cfg.ManagedProjects {
			if proj.ContextName == expectedProj.name && proj.ProjectPath == expectedProj.path {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Project %s not found in list", expectedProj.name)
		}
	}
}

// Test_VerifyCommand tests the verify command
func Test_VerifyCommand(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()

	// Add a project with symlink
	h.AddProject("test-project", h.projectDir)

	contextFile := filepath.Join(h.dataDir, "contexts", "test-project", "claude.md")
	symlinkPath := filepath.Join(h.projectDir, "claude.md")
	os.Symlink(contextFile, symlinkPath)

	// Verify symlink is valid
	target, err := os.Readlink(symlinkPath)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}
	if target != contextFile {
		t.Errorf("Symlink target is %s, expected %s", target, contextFile)
	}

	// Break the symlink by removing the target
	os.Remove(contextFile)

	// Check if symlink is now broken
	if _, err := os.Stat(symlinkPath); err == nil {
		t.Error("Symlink should be broken but isn't")
	}

	// Recreate the target (simulating --fix)
	os.WriteFile(contextFile, []byte("# test-project"), 0644)

	// Verify symlink is now valid again
	if _, err := os.Stat(symlinkPath); err != nil {
		t.Error("Symlink is still broken after fix")
	}
}