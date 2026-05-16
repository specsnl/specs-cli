package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	pkgtemplate "github.com/specsnl/specs-cli/pkg/template"
)

func TestUpdate_NoArgs_EmptyRegistry(t *testing.T) {
	withTempRegistry(t)

	_, err := executeCmd("template", "update")
	if err != nil {
		t.Fatalf("template update with empty registry: %v", err)
	}
}

func TestUpdate_NamedLocalTemplate_Skipped(t *testing.T) {
	registryDir := withTempRegistry(t)

	// Create a local template (no Repository/Branch in metadata).
	tmplDir := filepath.Join(registryDir, "local-tpl")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := pkgtemplate.SaveMetadata(tmplDir, "local-tpl", "/some/local/path", "", "", "", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	// Should succeed: local templates are silently skipped.
	_, err := executeCmd("template", "update", "local-tpl")
	if err != nil {
		t.Fatalf("template update local-tpl: %v", err)
	}
}

func TestUpdate_TooManyArgs(t *testing.T) {
	withTempRegistry(t)

	_, err := executeCmd("template", "update", "a", "b")
	if err == nil {
		t.Fatal("expected error when too many args given")
	}
}

func TestUpdate_NamedLocalTemplate_ProducesNoOutput(t *testing.T) {
	registryDir := withTempRegistry(t)

	// A template with a local Repository but no Branch — silently skipped.
	tmplDir := filepath.Join(registryDir, "local-tpl")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := pkgtemplate.SaveMetadata(tmplDir, "local-tpl", "/some/local/path", "", "", "", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	out, err := executeCmd("template", "update", "local-tpl")
	if err != nil {
		t.Fatalf("template update local-tpl: %v", err)
	}
	if out != "" {
		t.Errorf("expected no output for local/skipped template, got: %q", out)
	}
}

func TestUpdate_NoArgs_EmptyRegistry_ProducesNoOutput(t *testing.T) {
	withTempRegistry(t)

	out, err := executeCmd("template", "update")
	if err != nil {
		t.Fatalf("template update with empty registry: %v", err)
	}
	if out != "" {
		t.Errorf("expected no output for empty registry, got: %q", out)
	}
}
