package clauderc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ClaudeRC represents a .clauderc configuration file
type ClaudeRC struct {
	AdditionalContext []string `json:"additionalContext"`
}

// Manager handles .clauderc file operations
type Manager struct {
	projectPath string
	rcPath      string
}

// NewManager creates a new .clauderc manager for a project directory
func NewManager(projectPath string) *Manager {
	return &Manager{
		projectPath: projectPath,
		rcPath:      filepath.Join(projectPath, ".clauderc"),
	}
}

// Exists checks if .clauderc file exists
func (m *Manager) Exists() bool {
	_, err := os.Stat(m.rcPath)
	return err == nil
}

// Load reads and parses the .clauderc file
func (m *Manager) Load() (*ClaudeRC, error) {
	data, err := os.ReadFile(m.rcPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty ClaudeRC if file doesn't exist
			return &ClaudeRC{AdditionalContext: []string{}}, nil
		}
		return nil, fmt.Errorf("failed to read .clauderc: %w", err)
	}

	var rc ClaudeRC
	if err := json.Unmarshal(data, &rc); err != nil {
		return nil, fmt.Errorf("failed to parse .clauderc: %w", err)
	}

	return &rc, nil
}

// Save writes the .clauderc file
func (m *Manager) Save(rc *ClaudeRC) error {
	data, err := json.MarshalIndent(rc, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal .clauderc: %w", err)
	}

	// Write to temp file first (atomic operation)
	tempFile := m.rcPath + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp .clauderc: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempFile, m.rcPath); err != nil {
		os.Remove(tempFile) // Clean up temp file
		return fmt.Errorf("failed to save .clauderc: %w", err)
	}

	return nil
}

// AddFile adds a file to .clauderc, creating the file if needed
func (m *Manager) AddFile(fileName string, dryRun bool) error {
	// Check if context files exist and should be included
	initialFiles := []string{}

	if !m.Exists() {
		// Check for existing context files when creating .clauderc
		claudeMD := filepath.Join(m.projectPath, "claude.md")
		globalMD := filepath.Join(m.projectPath, "global.md")

		if _, err := os.Stat(claudeMD); err == nil {
			initialFiles = append(initialFiles, "claude.md")
		}
		if _, err := os.Stat(globalMD); err == nil {
			initialFiles = append(initialFiles, "global.md")
		}
	}

	if dryRun {
		if !m.Exists() {
			fmt.Printf("[DRY RUN] Would create .clauderc: %s\n", m.rcPath)
			for _, f := range initialFiles {
				fmt.Printf("[DRY RUN] Would include %s in .clauderc\n", f)
			}
		}
		fmt.Printf("[DRY RUN] Would add '%s' to .clauderc\n", fileName)
		return nil
	}

	rc, err := m.Load()
	if err != nil {
		return err
	}

	// Add initial files if this is a new .clauderc
	if !m.Exists() {
		rc.AdditionalContext = append(rc.AdditionalContext, initialFiles...)
	}

	// Check if file is already in the list
	for _, f := range rc.AdditionalContext {
		if f == fileName {
			// Already exists, no need to add
			return nil
		}
	}

	// Add the new file
	rc.AdditionalContext = append(rc.AdditionalContext, fileName)

	return m.Save(rc)
}

// RemoveFile removes a file from .clauderc
func (m *Manager) RemoveFile(fileName string, dryRun bool) error {
	if !m.Exists() {
		// Nothing to remove
		return nil
	}

	if dryRun {
		fmt.Printf("[DRY RUN] Would remove '%s' from .clauderc\n", fileName)
		return nil
	}

	rc, err := m.Load()
	if err != nil {
		return err
	}

	// Remove the file from the list
	newContext := []string{}
	for _, f := range rc.AdditionalContext {
		if f != fileName {
			newContext = append(newContext, f)
		}
	}

	rc.AdditionalContext = newContext

	return m.Save(rc)
}

// Contains checks if a file is referenced in .clauderc
func (m *Manager) Contains(fileName string) (bool, error) {
	if !m.Exists() {
		return false, nil
	}

	rc, err := m.Load()
	if err != nil {
		return false, err
	}

	for _, f := range rc.AdditionalContext {
		if f == fileName {
			return true, nil
		}
	}

	return false, nil
}

// List returns all files referenced in .clauderc
func (m *Manager) List() ([]string, error) {
	if !m.Exists() {
		return []string{}, nil
	}

	rc, err := m.Load()
	if err != nil {
		return nil, err
	}

	return rc.AdditionalContext, nil
}
