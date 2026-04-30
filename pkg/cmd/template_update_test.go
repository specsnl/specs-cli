package cmd

import (
	"os"
	"path/filepath"
	"testing"
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
	if err := writeMetadata(tmplDir, "local-tpl", "/some/local/path", "", "", ""); err != nil {
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
