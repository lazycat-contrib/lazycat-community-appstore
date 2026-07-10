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
