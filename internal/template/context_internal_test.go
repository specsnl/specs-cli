package template

import (
	"testing"

	"github.com/specsnl/specs-cli/internal/specs"
)

func TestResolveReferencedDefaults_DoesNotMutateInput(t *testing.T) {
	input := map[string]any{
		"Name": "My App",
		"Slug": "{{ .Name | toKebabCase }}",
	}
	// Take a snapshot of the template expression before calling.
	originalSlug := input["Slug"]

	fm := FuncMap(Config{})

	result, err := resolveReferencedDefaults(input, fm, specs.DefaultDelimiters)
	if err != nil {
		t.Fatalf("resolveReferencedDefaults: %v", err)
	}

	// The input map must not have been modified.
	if input["Slug"] != originalSlug {
		t.Errorf("input[Slug] was mutated: got %q, want %q", input["Slug"], originalSlug)
	}

	// The returned map must have the resolved value.
	if result["Slug"] != "my-app" {
		t.Errorf("result[Slug] = %q, want %q", result["Slug"], "my-app")
	}
}

func TestResolveReferencedDefaults_NoRefs_ReturnsSameMap(t *testing.T) {
	input := map[string]any{"Name": "plain"}
	fm := FuncMap(Config{})

	result, err := resolveReferencedDefaults(input, fm, specs.DefaultDelimiters)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// When there are no template refs, the original map may be returned as-is.
	if result["Name"] != "plain" {
		t.Errorf("result[Name] = %q, want %q", result["Name"], "plain")
	}
}
