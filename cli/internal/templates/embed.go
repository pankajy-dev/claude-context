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

// GetTemplate reads a template by name, checking user dir first, then source dir (dev), then embedded
// Returns template content and source ("user", "dev", or "embedded")
func GetTemplate(name string, dataDir string) ([]byte, string, error) {
	// 1. Check user templates first (~/.cctx/templates/)
	userTemplatePath := filepath.Join(dataDir, "templates", name+".md")
	if content, err := os.ReadFile(userTemplatePath); err == nil {
		return content, "user", nil
	}

	// 2. Check source directory (for development) - cli/internal/templates/
	// This allows editing templates without rebuilding during development
	if cwd, err := os.Getwd(); err == nil {
		devTemplatePath := filepath.Join(cwd, "cli", "internal", "templates", name+".md")
		if content, err := os.ReadFile(devTemplatePath); err == nil {
			return content, "dev", nil
		}
	}

	// 3. Fall back to embedded template (production)
	content, err := embeddedTemplates.ReadFile(name + ".md")
	if err != nil {
		return nil, "", fmt.Errorf("template not found: %s", name)
	}
	return content, "embedded", nil
}

// ListTemplates returns all available templates (merged from user + embedded)
func ListTemplates(dataDir string) ([]Template, error) {
	templateMap := make(map[string]Template)

	// First, load embedded templates (defaults)
	entries, err := embeddedTemplates.ReadDir(".")
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			name := strings.TrimSuffix(entry.Name(), ".md")
			templateMap[name] = Template{
				Name:   name,
				Path:   "embedded",
				Source: "embedded",
			}
		}
	}

	// Then, override with user templates if they exist
	userTemplatesDir := filepath.Join(dataDir, "templates")
	if userEntries, err := os.ReadDir(userTemplatesDir); err == nil {
		for _, entry := range userEntries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
				name := strings.TrimSuffix(entry.Name(), ".md")
				templateMap[name] = Template{
					Name:   name,
					Path:   filepath.Join("templates", entry.Name()),
					Source: "user",
				}
			}
		}
	}

	// Convert map to slice
	templates := make([]Template, 0, len(templateMap))
	for _, tmpl := range templateMap {
		templates = append(templates, tmpl)
	}

	return templates, nil
}
