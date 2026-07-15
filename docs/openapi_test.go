package docs

import (
	"os"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestOpenAPIYAMLParses(t *testing.T) {
	data, err := os.ReadFile("openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var document any
	if err := yaml.Unmarshal(data, &document); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}
}

func TestOpenAPIDocumentsLazyCatServerInstallation(t *testing.T) {
	data, err := os.ReadFile("openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Paths map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(data, &document); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}
	for _, path := range []string{
		"/api/v1/runtime/capabilities",
		"/api/v1/apps/{id}/versions/{versionId}/install",
	} {
		if _, ok := document.Paths[path]; !ok {
			t.Fatalf("OpenAPI path %q is missing", path)
		}
	}
}
