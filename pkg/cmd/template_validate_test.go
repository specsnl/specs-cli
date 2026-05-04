package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/specsnl/specs-cli/pkg/util/exit"
)

func TestValidate_ValidTemplate(t *testing.T) {
	withTempRegistry(t)
	src := makeFakeTemplate(t)

	if _, err := executeCmd("template", "validate", src); err != nil {
		t.Fatalf("template validate: %v", err)
	}
}

func TestValidate_MissingTemplateDir(t *testing.T) {
	withTempRegistry(t)

	src := t.TempDir()
	_, err := executeCmd("template", "validate", src)
	if err == nil {
		t.Fatal("expected error for missing template/ subdir")
	}
}

// makeValidateTemplate creates a template with project.yaml and specific template files.
func makeValidateTemplate(t *testing.T, projectYAML string, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "project.yaml"), []byte(projectYAML), 0644); err != nil {
		t.Fatal(err)
	}
	templateDir := filepath.Join(dir, "template")
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		abs := filepath.Join(templateDir, name)
		if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func validateExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exit.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.Code
	}
	return -1 // unexpected error type
}

// TestValidate_UnusedVariable: warning printed, exit 0 (no --strict).
func TestValidate_UnusedVariable(t *testing.T) {
	withTempRegistry(t)
	src := makeValidateTemplate(t,
		"Name: \"\"\nDatabasePort: \"5432\"\n",
		map[string]string{"main.go": "package [[.Name]]"},
	)

	out, err := executeCmd("template", "validate", src)
	if err != nil {
		t.Fatalf("expected exit 0, got error: %v", err)
	}
	if !strings.Contains(out, "DatabasePort") {
		t.Errorf("expected warning about DatabasePort, got: %q", out)
	}
	if !strings.Contains(out, "defined but never used") {
		t.Errorf("expected 'defined but never used' in output, got: %q", out)
	}
}

// TestValidate_UnusedComputed: warning printed, exit 0 (no --strict).
func TestValidate_UnusedComputed(t *testing.T) {
	withTempRegistry(t)
	src := makeValidateTemplate(t,
		"Name: \"\"\ncomputed:\n  Slug: \"[[toKebabCase .Name]]\"\n",
		map[string]string{"main.go": "package [[.Name]]"},
	)

	out, err := executeCmd("template", "validate", src)
	if err != nil {
		t.Fatalf("expected exit 0, got error: %v", err)
	}
	if !strings.Contains(out, "Slug") {
		t.Errorf("expected warning about Slug, got: %q", out)
	}
	if !strings.Contains(out, "defined but never used") {
		t.Errorf("expected 'defined but never used' in output, got: %q", out)
	}
}

// TestValidate_UnknownVariable: warning printed, exit 2.
func TestValidate_UnknownVariable(t *testing.T) {
	withTempRegistry(t)
	src := makeValidateTemplate(t,
		"Name: \"\"\n",
		map[string]string{"main.go": "package [[.Name]]\n// author: [[.AppName]]"},
	)

	out, err := executeCmd("template", "validate", src)
	if code := validateExitCode(err); code != exit.ValidateUnknown {
		t.Errorf("expected exit %d, got %d (err=%v)", exit.ValidateUnknown, code, err)
	}
	if !strings.Contains(out, "AppName") {
		t.Errorf("expected warning about AppName, got: %q", out)
	}
	if !strings.Contains(out, "not defined in project.yaml") {
		t.Errorf("expected 'not defined in project.yaml' in output, got: %q", out)
	}
}

// TestValidate_StrictUnusedVariable: --strict with unused variable → exit 1.
func TestValidate_StrictUnusedVariable(t *testing.T) {
	withTempRegistry(t)
	src := makeValidateTemplate(t,
		"Name: \"\"\nDatabasePort: \"5432\"\n",
		map[string]string{"main.go": "package [[.Name]]"},
	)

	_, err := executeCmd("template", "validate", "--strict", src)
	if code := validateExitCode(err); code != exit.ValidateUnused {
		t.Errorf("expected exit %d, got %d (err=%v)", exit.ValidateUnused, code, err)
	}
}

// TestValidate_StrictUnusedAndUnknown: --strict with both → exit 3 (bitmask 1|2).
func TestValidate_StrictUnusedAndUnknown(t *testing.T) {
	withTempRegistry(t)
	src := makeValidateTemplate(t,
		"Name: \"\"\nDatabasePort: \"5432\"\n",
		map[string]string{"main.go": "package [[.Name]]\n// [[.AppName]]"},
	)

	_, err := executeCmd("template", "validate", "--strict", src)
	const wantCode = exit.ValidateUnused | exit.ValidateUnknown // 3
	if code := validateExitCode(err); code != wantCode {
		t.Errorf("expected exit %d, got %d (err=%v)", wantCode, code, err)
	}
}

// TestValidate_NoIssues: clean template → "template is valid", exit 0.
func TestValidate_NoIssues(t *testing.T) {
	withTempRegistry(t)
	src := makeValidateTemplate(t,
		"Name: \"\"\n",
		map[string]string{"main.go": "package [[.Name]]"},
	)

	out, err := executeCmd("template", "validate", src)
	if err != nil {
		t.Fatalf("expected exit 0, got error: %v", err)
	}
	if !strings.Contains(out, "template is valid") {
		t.Errorf("expected 'template is valid', got: %q", out)
	}
}
