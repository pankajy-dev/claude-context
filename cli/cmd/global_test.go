package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pankaj/claude-context/internal/config"
)

// Test_GlobalCreate tests creating a global context
func Test_GlobalCreate(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()

	// Create a global context
	globalName := "script"
	description := "Shell scripting guidelines"

	cfg := h.GetConfig()
	globalContext := config.GlobalContext{
		Name:        globalName,
		Description: description,
		Path:        filepath.Join("contexts", "_global", globalName+".md"),
		Enabled:     true,
	}

	cfg.GlobalContexts = append(cfg.GlobalContexts, globalContext)
	if err := h.configMgr.Save(cfg); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Create global context file
	globalDir := filepath.Join(h.dataDir, "contexts", "_global")
	globalFile := filepath.Join(globalDir, globalName+".md")
	if err := os.WriteFile(globalFile, []byte("# "+globalName+" guidelines"), 0644); err != nil {
		t.Fatalf("Failed to create global context file: %v", err)
	}

	// Verify global context was added to config
	cfg = h.GetConfig()
	found := false
	for _, gc := range cfg.GlobalContexts {
		if gc.Name == globalName {
			found = true
			if gc.Description != description {
				t.Errorf("Description is %s, expected %s", gc.Description, description)
			}
			if !gc.Enabled {
				t.Error("Global context should be enabled")
			}
			break
		}
	}
	if !found {
		t.Error("Global context not found in config")
	}

	// Verify global context file exists
	if _, err := os.Stat(globalFile); os.IsNotExist(err) {
		t.Error("Global context file does not exist")
	}
}

// Test_GlobalEnable tests enabling a global context
func Test_GlobalEnable(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()

	// Create a disabled global context
	globalName := "python"
	cfg := h.GetConfig()

	globalContext := config.GlobalContext{
		Name:        globalName,
		Description: "Python guidelines",
		Path:        filepath.Join("contexts", "_global", globalName+".md"),
		Enabled:     false,
	}

	cfg.GlobalContexts = append(cfg.GlobalContexts, globalContext)
	h.configMgr.Save(cfg)

	// Enable it
	cfg = h.GetConfig()
	for i := range cfg.GlobalContexts {
		if cfg.GlobalContexts[i].Name == globalName {
			cfg.GlobalContexts[i].Enabled = true
			break
		}
	}
	h.configMgr.Save(cfg)

	// Verify it's enabled
	cfg = h.GetConfig()
	for _, gc := range cfg.GlobalContexts {
		if gc.Name == globalName {
			if !gc.Enabled {
				t.Error("Global context should be enabled")
			}
			return
		}
	}
	t.Error("Global context not found")
}

// Test_GlobalDisable tests disabling a global context
func Test_GlobalDisable(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()

	// Create an enabled global context
	globalName := "go"
	cfg := h.GetConfig()

	globalContext := config.GlobalContext{
		Name:        globalName,
		Description: "Go guidelines",
		Path:        filepath.Join("contexts", "_global", globalName+".md"),
		Enabled:     true,
	}

	cfg.GlobalContexts = append(cfg.GlobalContexts, globalContext)
	h.configMgr.Save(cfg)

	// Disable it
	cfg = h.GetConfig()
	for i := range cfg.GlobalContexts {
		if cfg.GlobalContexts[i].Name == globalName {
			cfg.GlobalContexts[i].Enabled = false
			break
		}
	}
	h.configMgr.Save(cfg)

	// Verify it's disabled
	cfg = h.GetConfig()
	for _, gc := range cfg.GlobalContexts {
		if gc.Name == globalName {
			if gc.Enabled {
				t.Error("Global context should be disabled")
			}
			return
		}
	}
	t.Error("Global context not found")
}

// Test_GlobalLinkToProject tests linking a global context to a project
func Test_GlobalLinkToProject(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()

	// Create a project
	h.AddProject("test-project", h.projectDir)

	// Create a global context
	globalName := "docker"
	cfg := h.GetConfig()

	globalContext := config.GlobalContext{
		Name:        globalName,
		Description: "Docker guidelines",
		Path:        filepath.Join("contexts", "_global", globalName+".md"),
		Enabled:     true,
	}

	cfg.GlobalContexts = append(cfg.GlobalContexts, globalContext)
	h.configMgr.Save(cfg)

	// Create global context file
	globalFile := filepath.Join(h.dataDir, "contexts", "_global", globalName+".md")
	os.WriteFile(globalFile, []byte("# "+globalName+" guidelines"), 0644)

	// Link global to project
	cfg = h.GetConfig()
	for i := range cfg.ManagedProjects {
		if cfg.ManagedProjects[i].ContextName == "test-project" {
			cfg.ManagedProjects[i].LinkedGlobals = append(cfg.ManagedProjects[i].LinkedGlobals, globalName)
			break
		}
	}
	h.configMgr.Save(cfg)

	// Create symlink in project
	symlinkPath := filepath.Join(h.projectDir, ".claude", "_global", globalName+".md")
	os.MkdirAll(filepath.Dir(symlinkPath), 0755)
	os.Symlink(globalFile, symlinkPath)

	// Verify global is linked to project
	cfg = h.GetConfig()
	var project *config.Project
	for i := range cfg.ManagedProjects {
		if cfg.ManagedProjects[i].ContextName == "test-project" {
			project = &cfg.ManagedProjects[i]
			break
		}
	}
	if project == nil {
		t.Fatal("Project not found")
	}

	found := false
	for _, linkedGlobal := range project.LinkedGlobals {
		if linkedGlobal == globalName {
			found = true
			break
		}
	}
	if !found {
		t.Error("Global context not linked to project")
	}

	// Verify symlink exists
	if _, err := os.Lstat(symlinkPath); err != nil {
		t.Error("Symlink to global context does not exist")
	}
}

// Test_GlobalList tests listing global contexts
func Test_GlobalList(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()

	// Create multiple global contexts
	globals := []struct {
		name        string
		description string
		enabled     bool
	}{
		{"script", "Shell scripting", true},
		{"python", "Python guidelines", false},
		{"go", "Go guidelines", true},
	}

	cfg := h.GetConfig()
	for _, g := range globals {
		gc := config.GlobalContext{
			Name:        g.name,
			Description: g.description,
			Path:        filepath.Join("contexts", "_global", g.name+".md"),
			Enabled:     g.enabled,
		}
		cfg.GlobalContexts = append(cfg.GlobalContexts, gc)
	}
	h.configMgr.Save(cfg)

	// List global contexts
	cfg = h.GetConfig()

	if len(cfg.GlobalContexts) != 3 {
		t.Errorf("Expected 3 global contexts, got %d", len(cfg.GlobalContexts))
	}

	// Verify each global context
	for _, expected := range globals {
		found := false
		for _, gc := range cfg.GlobalContexts {
			if gc.Name == expected.name {
				found = true
				if gc.Description != expected.description {
					t.Errorf("Description for %s is %s, expected %s", expected.name, gc.Description, expected.description)
				}
				if gc.Enabled != expected.enabled {
					t.Errorf("Enabled status for %s is %v, expected %v", expected.name, gc.Enabled, expected.enabled)
				}
				break
			}
		}
		if !found {
			t.Errorf("Global context %s not found", expected.name)
		}
	}
}

// Test_GlobalUnlinkFromProject tests unlinking a global context from a project
func Test_GlobalUnlinkFromProject(t *testing.T) {
	h := NewTestHelper(t)
	defer h.Cleanup()

	// Create project and global context
	h.AddProject("test-project", h.projectDir)

	globalName := "rust"
	cfg := h.GetConfig()

	globalContext := config.GlobalContext{
		Name:        globalName,
		Description: "Rust guidelines",
		Path:        filepath.Join("contexts", "_global", globalName+".md"),
		Enabled:     true,
	}

	cfg.GlobalContexts = append(cfg.GlobalContexts, globalContext)

	// Link global to project
	for i := range cfg.ManagedProjects {
		if cfg.ManagedProjects[i].ContextName == "test-project" {
			cfg.ManagedProjects[i].LinkedGlobals = []string{globalName}
			break
		}
	}
	h.configMgr.Save(cfg)

	// Create symlink
	globalFile := filepath.Join(h.dataDir, "contexts", "_global", globalName+".md")
	os.WriteFile(globalFile, []byte("# "+globalName), 0644)

	symlinkPath := filepath.Join(h.projectDir, ".claude", "_global", globalName+".md")
	os.MkdirAll(filepath.Dir(symlinkPath), 0755)
	os.Symlink(globalFile, symlinkPath)

	// Unlink global from project
	cfg = h.GetConfig()
	for i := range cfg.ManagedProjects {
		if cfg.ManagedProjects[i].ContextName == "test-project" {
			cfg.ManagedProjects[i].LinkedGlobals = []string{}
			break
		}
	}
	h.configMgr.Save(cfg)

	// Remove symlink
	os.Remove(symlinkPath)

	// Verify global is no longer linked
	cfg = h.GetConfig()
	var project *config.Project
	for i := range cfg.ManagedProjects {
		if cfg.ManagedProjects[i].ContextName == "test-project" {
			project = &cfg.ManagedProjects[i]
			break
		}
	}
	if project == nil {
		t.Fatal("Project not found")
	}

	if len(project.LinkedGlobals) != 0 {
		t.Errorf("Expected 0 linked globals, got %d", len(project.LinkedGlobals))
	}

	// Verify symlink was removed
	if _, err := os.Lstat(symlinkPath); err == nil {
		t.Error("Symlink still exists after unlink")
	}
}