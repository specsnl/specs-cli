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

func TestUpgrade_AllFlagMutualExclusion(t *testing.T) {
	withTempRegistry(t)

	_, err := executeCmd("template", "upgrade", "--all", "mytemplate")
	if err == nil {
		t.Fatal("expected error when both --all and a name are given")
	}
}

func TestUpgrade_NeitherAllNorName(t *testing.T) {
	withTempRegistry(t)

	_, err := executeCmd("template", "upgrade")
	if err == nil {
		t.Fatal("expected error when neither --all nor a name is given")
	}
}

func TestUpgrade_NonexistentTemplate(t *testing.T) {
	withTempRegistry(t)

	_, err := executeCmd("template", "upgrade", "does-not-exist")
	if err == nil {
		t.Fatal("expected error for non-existent template")
	}
}
