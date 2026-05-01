package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUpgrade_LocalSkipped(t *testing.T) {
	registryDir := withTempRegistry(t)

	// Create a local template (no Branch in metadata).
	tmplDir := filepath.Join(registryDir, "local-tpl")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := writeMetadata(tmplDir, "local-tpl", "/some/local/path", "", "", ""); err != nil {
		t.Fatal(err)
	}

	// Should succeed: local templates are skipped with a notice.
	_, err := executeCmd("template", "upgrade", "local-tpl")
	if err != nil {
		t.Fatalf("template upgrade local-tpl: %v", err)
	}
}

func TestUpgrade_NoArgs_EmptyRegistry(t *testing.T) {
	withTempRegistry(t)

	// No args on an empty registry should succeed (nothing to upgrade).
	_, err := executeCmd("template", "upgrade")
	if err != nil {
		t.Fatalf("template upgrade with no args on empty registry: %v", err)
	}
}

func TestUpgrade_NonexistentTemplate(t *testing.T) {
	withTempRegistry(t)

	_, err := executeCmd("template", "upgrade", "does-not-exist")
	if err == nil {
		t.Fatal("expected error for non-existent template")
	}
}
