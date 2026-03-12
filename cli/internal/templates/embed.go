package templates

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Embed templates from repository (source of truth)
//
//go:embed *.md
var embeddedTemplates embed.FS

// Template represents a template with its source location
type Template struct {
	Name   string
	Path   string
	Source string // "user" or "embedded"
}

// GetTemplateFS returns the embedded filesystem containing templates
func GetTemplateFS() fs.FS {
	return embeddedTemplates
}

// GetTemplate reads a template by name from user directory
// Falls back to embedded if not found (for safety during migration)
func GetTemplate(name string, dataDir string) ([]byte, string, error) {
	// Primary: Check user templates (~/.cctx/templates/)
	// This is the single source of truth after initialization
	userTemplatePath := filepath.Join(dataDir, "templates", name+".md")
	if content, err := os.ReadFile(userTemplatePath); err == nil {
		return content, "user", nil
	}

	// Fallback: embedded template (for safety during migration or if templates dir is missing)
	content, err := embeddedTemplates.ReadFile(name + ".md")
	if err != nil {
		return nil, "", fmt.Errorf("template not found: %s", name)
	}
	return content, "embedded", nil
}

// CopyEmbeddedTemplate copies a specific embedded template to the user directory
func CopyEmbeddedTemplate(name string, dataDir string) error {
	// Read from embedded
	content, err := embeddedTemplates.ReadFile(name + ".md")
	if err != nil {
		return fmt.Errorf("template not found in embedded: %s", name)
	}

	// Ensure templates directory exists
	templatesDir := filepath.Join(dataDir, "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		return fmt.Errorf("failed to create templates directory: %w", err)
	}

	// Write to user directory
	userTemplatePath := filepath.Join(templatesDir, name+".md")
	if err := os.WriteFile(userTemplatePath, content, 0644); err != nil {
		return fmt.Errorf("failed to write template: %w", err)
	}

	return nil
}

// CopyAllEmbeddedTemplates copies all embedded templates to user directory
func CopyAllEmbeddedTemplates(dataDir string, overwrite bool) (int, error) {
	entries, err := embeddedTemplates.ReadDir(".")
	if err != nil {
		return 0, fmt.Errorf("failed to read embedded templates: %w", err)
	}

	templatesDir := filepath.Join(dataDir, "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		return 0, fmt.Errorf("failed to create templates directory: %w", err)
	}

	copied := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".md")
		userTemplatePath := filepath.Join(templatesDir, entry.Name())

		// Skip if exists and overwrite is false
		if !overwrite {
			if _, err := os.Stat(userTemplatePath); err == nil {
				continue
			}
		}

		if err := CopyEmbeddedTemplate(name, dataDir); err != nil {
			return copied, fmt.Errorf("failed to copy %s: %w", name, err)
		}
		copied++
	}

	return copied, nil
}

// ListTemplates returns all available templates from user directory
// Falls back to embedded list if user templates directory doesn't exist
func ListTemplates(dataDir string) ([]Template, error) {
	templates := []Template{}

	// Primary: Load from user templates directory (single source of truth)
	userTemplatesDir := filepath.Join(dataDir, "templates")
	if userEntries, err := os.ReadDir(userTemplatesDir); err == nil {
		for _, entry := range userEntries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
				name := strings.TrimSuffix(entry.Name(), ".md")
				templates = append(templates, Template{
					Name:   name,
					Path:   filepath.Join("templates", entry.Name()),
					Source: "user",
				})
			}
		}
		return templates, nil
	}

	// Fallback: If user templates don't exist, list embedded (shouldn't happen after init)
	entries, err := embeddedTemplates.ReadDir(".")
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			name := strings.TrimSuffix(entry.Name(), ".md")
			templates = append(templates, Template{
				Name:   name,
				Path:   "embedded",
				Source: "embedded",
			})
		}
	}

	return templates, nil
}
