package template

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/infrastructure-io/topohub/pkg/log"
)

// TemplateData contains the data needed to render the templates
type TemplateData struct {
	Name               string           `json:"name"`
	Namespace          string           `json:"namespace"`
	ClusterName        string           `json:"clusterName"`
	Image              string           `json:"image"`
	Replicas           int32            `json:"replicas"`
	ServiceAccountName string           `json:"serviceAccountName"`
	RoleName           string           `json:"roleName"`
	UnderlayInterface  string           `json:"underlayInterface"`
	NodeAffinity       *v1.NodeAffinity `json:"nodeAffinity,omitempty"`
	NodeName           string           `json:"nodeName"`
	HostNetwork        bool             `json:"hostNetwork"`
}

// toYaml takes an interface, marshals it to yaml, and returns a string
func toYaml(v interface{}) string {
	data, err := yaml.Marshal(v)
	if err != nil {
		// Swallow errors inside of a template.
		return ""
	}
	return string(data)
}

// ShouldHandlePVC 检查是否需要处理 PVC
func ShouldHandlePVC() bool {
	pvcPath := filepath.Join("/etc/bmc/templates", "agent-pvc.yaml")
	_, err := os.Stat(pvcPath)
	return err == nil
}

// RenderTemplate reads a template file and renders it with the given data
func RenderTemplate(templateName string, data *TemplateData) (*unstructured.Unstructured, error) {
	log.Logger.Infof("Starting template rendering: %s", templateName)

	templatePath := filepath.Join("/etc/bmc/templates", templateName)
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		log.Logger.Errorf("Failed to read template file %s: %v", templatePath, err)
		return nil, fmt.Errorf("failed to read template file %s: %v", templatePath, err)
	}

	log.Logger.Debugf("Template file read successfully: %s (size: %d)", templatePath, len(templateContent))

	funcMap := template.FuncMap{
		"toYaml": toYaml,
	}

	tmpl, err := template.New(templateName).Funcs(funcMap).Parse(string(templateContent))
	if err != nil {
		log.Logger.Errorf("Failed to parse template %s: %v", templateName, err)
		return nil, fmt.Errorf("failed to parse template %s: %v", templateName, err)
	}

	log.Logger.Debugf("Template parsed successfully: %s", templateName)

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		log.Logger.Errorf("Failed to execute template %s: %v", templateName, err)
		return nil, fmt.Errorf("failed to execute template %s: %v", templateName, err)
	}

	log.Logger.Debugf("Template executed successfully: %s (output size: %d)", templateName, buf.Len())

	var obj map[string]interface{}
	err = yaml.Unmarshal(buf.Bytes(), &obj)
	if err != nil {
		log.Logger.Errorf("Failed to unmarshal template output %s: %v", templateName, err)
		return nil, fmt.Errorf("failed to unmarshal template output %s: %v", templateName, err)
	}

	result := &unstructured.Unstructured{Object: obj}

	log.Logger.Infof("Template rendered successfully: %s (kind: %s, name: %s, namespace: %s)",
		templateName, result.GetKind(), result.GetName(), result.GetNamespace())

	return result, nil
}

// RenderAllAgentResources renders all the resources needed for an agent
func RenderAllAgentResources(data *TemplateData) (map[string]*unstructured.Unstructured, error) {
	log.Logger.Infof("Starting to render all agent resources for cluster: %s", data.ClusterName)

	resources := make(map[string]*unstructured.Unstructured)
	templates := []string{
		"agent-deployment.yaml",
		"agent-serviceaccount.yaml",
		"agent-clusterrole.yaml",
		"agent-clusterrolebinding.yaml",
	}

	// Check if agent-pvc.yaml exists and try to render it
	if ShouldHandlePVC() {
		if obj, err := RenderTemplate("agent-pvc.yaml", data); err == nil {
			kind := strings.ToLower(obj.GetKind())
			resources[kind] = obj
			log.Logger.Infof("Successfully rendered PVC template")
		} else {
			log.Logger.Infof("Failed to render PVC template: %v", err)
		}
	}

	// Render other templates
	for _, tmpl := range templates {
		log.Logger.Infof("Rendering template: %s", tmpl)

		obj, err := RenderTemplate(tmpl, data)
		if err != nil {
			log.Logger.Errorf("Failed to render template %s: %v", tmpl, err)
			return nil, err
		}

		kind := strings.ToLower(obj.GetKind())
		resources[kind] = obj

		log.Logger.Debugf("Template rendered and added to resources: %s (kind: %s, name: %s, namespace: %s)",
			tmpl, obj.GetKind(), obj.GetName(), obj.GetNamespace())
	}

	log.Logger.Infof("Successfully rendered %d agent resources", len(resources))

	return resources, nil
}
