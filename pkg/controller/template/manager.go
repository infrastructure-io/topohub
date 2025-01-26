package template

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

const (
	TemplateDir = "/etc/bmc/templates"
)

// Manager handles template operations
type Manager struct {
	templateDir string
}

// NewManager creates a new template manager
func NewManager(templateDir string) *Manager {
	return &Manager{
		templateDir: templateDir,
	}
}

// RenderYAML renders a YAML template with the given data
func (m *Manager) RenderYAML(templateName string, data map[string]interface{}) (*unstructured.Unstructured, error) {
	templatePath := filepath.Join(m.templateDir, templateName)
	content, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read template %s: %v", templatePath, err)
	}

	// Create a new template and parse the content
	tmpl, err := template.New(templateName).Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template %s: %v", templateName, err)
	}

	// Execute the template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("failed to execute template %s: %v", templateName, err)
	}

	// Parse the rendered YAML into an unstructured object
	var obj unstructured.Unstructured
	if err := yaml.Unmarshal(buf.Bytes(), &obj); err != nil {
		return nil, fmt.Errorf("failed to unmarshal rendered template %s: %v", templateName, err)
	}

	return &obj, nil
}
