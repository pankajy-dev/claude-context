package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Config represents the main configuration structure
type Config struct {
	SchemaVersion   int             `json:"schema_version,omitempty"` // 1=old, 2=new (default 1 for backwards compat)
	ManagedProjects []Project       `json:"managed_projects"`
	GlobalContexts  []GlobalContext `json:"global_contexts"`
	Tickets         TicketSection   `json:"tickets"`
	Settings        Settings        `json:"settings"`
}

// Project represents a managed project
type Project struct {
	ContextName   string    `json:"context_name"`
	ProjectPath   string    `json:"project_path"`
	ContextPath   string    `json:"context_path"`
	CreatedAt     time.Time `json:"created_at"`
	LastModified  time.Time `json:"last_modified,omitempty"`
	Status        string    `json:"symlink_status,omitempty"`
	LinkedGlobals []string  `json:"linked_globals,omitempty"`
}

// GlobalContext represents a global context file
type GlobalContext struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"path"`
	Enabled     bool   `json:"enabled"`
}

// TicketSection contains ticket-related configuration
type TicketSection struct {
	Active   []Ticket `json:"active"`
	Archived []Ticket `json:"archived"`
	Settings TicketSettings `json:"settings"`
}

// Ticket represents a ticket workspace
type Ticket struct {
	TicketID           string          `json:"ticket_id"`
	Title              string          `json:"title"`
	Status             string          `json:"status"`
	CreatedAt          time.Time       `json:"created_at"`
	LastModified       time.Time       `json:"last_modified"`
	CompletedAt        *time.Time      `json:"completed_at,omitempty"`
	AbandonedAt        *time.Time      `json:"abandoned_at,omitempty"`
	LinkedProjects     []LinkedProject `json:"linked_projects"`
	Tags               []string        `json:"tags"`
	Notes              string          `json:"notes"`
	Commits            []string        `json:"commits,omitempty"`
	PullRequests       []string        `json:"pull_requests,omitempty"`
	ArchivedPath       string          `json:"archived_path,omitempty"`
	DocGenerated       bool            `json:"doc_generated,omitempty"`
	PrimaryContextName string          `json:"primary_context_name,omitempty"` // Which project has concrete files (v2 schema)
}

// LinkedProject represents a project linked to a ticket
type LinkedProject struct {
	ContextName string `json:"context_name"`
	ProjectPath string `json:"project_path"`
}

// TicketSettings contains ticket-specific settings
type TicketSettings struct {
	AutoArchive bool `json:"auto_archive"`
}

// Settings contains global settings
type Settings struct {
	AutoCommit      bool `json:"auto_commit"`
	BackupOnUnlink  bool `json:"backup_on_unlink"`
}

// Manager handles config file operations
type Manager struct {
	configPath   string
	contextsPath string
	repoRoot     string
}

// NewManager creates a new config manager
func NewManager(repoRoot string) *Manager {
	return &Manager{
		configPath:   filepath.Join(repoRoot, "config.json"),
		contextsPath: filepath.Join(repoRoot, "contexts"),
		repoRoot:     repoRoot,
	}
}

// Load reads and parses the config file
func (m *Manager) Load() (*Config, error) {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Default to schema version 1 for backwards compatibility
	if config.SchemaVersion == 0 {
		config.SchemaVersion = 1
	}

	return &config, nil
}

// Save writes the config to file
func (m *Manager) Save(config *Config) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to temp file first (atomic operation)
	tempFile := m.configPath + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp config: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempFile, m.configPath); err != nil {
		os.Remove(tempFile) // Clean up temp file
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

// GetProject finds a project by context name
func (c *Config) GetProject(contextName string) *Project {
	for i := range c.ManagedProjects {
		if c.ManagedProjects[i].ContextName == contextName {
			return &c.ManagedProjects[i]
		}
	}
	return nil
}

// AddProject adds a new project to the config
func (c *Config) AddProject(project Project) {
	c.ManagedProjects = append(c.ManagedProjects, project)
}

// RemoveProject removes a project from the config
func (c *Config) RemoveProject(contextName string) bool {
	for i, p := range c.ManagedProjects {
		if p.ContextName == contextName {
			c.ManagedProjects = append(c.ManagedProjects[:i], c.ManagedProjects[i+1:]...)
			return true
		}
	}
	return false
}

// GetTicket finds a ticket by ID
func (c *Config) GetTicket(ticketID string, includeArchived bool) *Ticket {
	for i := range c.Tickets.Active {
		if c.Tickets.Active[i].TicketID == ticketID {
			return &c.Tickets.Active[i]
		}
	}

	if includeArchived {
		for i := range c.Tickets.Archived {
			if c.Tickets.Archived[i].TicketID == ticketID {
				return &c.Tickets.Archived[i]
			}
		}
	}

	return nil
}

// GetGlobalContext finds a global context by name
func (c *Config) GetGlobalContext(name string) *GlobalContext {
	for i := range c.GlobalContexts {
		if c.GlobalContexts[i].Name == name {
			return &c.GlobalContexts[i]
		}
	}
	return nil
}

// GetRepoRoot returns the repository root path
func (m *Manager) GetRepoRoot() string {
	return m.repoRoot
}

// GetContextsPath returns the contexts directory path
func (m *Manager) GetContextsPath() string {
	return m.contextsPath
}
