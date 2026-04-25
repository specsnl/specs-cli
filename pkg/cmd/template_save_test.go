package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/specsnl/specs-cli/pkg/specs"
)

// makeFakeTemplate creates a minimal template directory structure in dir.
func makeFakeTemplate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, specs.TemplateDirFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, specs.ProjectYAMLFile), []byte("variables: []\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestSave_Success(t *testing.T) {
	withTempRegistry(t)

	src := makeFakeTemplate(t)
	if _, err := executeCmd("template", "save", src, "my-tag"); err != nil {
		t.Fatalf("template save: %v", err)
	}

	if _, err := os.Stat(specs.TemplatePath("my-tag")); err != nil {
		t.Errorf("expected registry entry to exist: %v", err)
	}
}

func TestSave_AlreadyExists(t *testing.T) {
	withTempRegistry(t)

	src := makeFakeTemplate(t)
	if _, err := executeCmd("template", "save", src, "my-tag"); err != nil {
		t.Fatal(err)
	}

	_, err := executeCmd("template", "save", src, "my-tag")
	if err == nil {
		t.Fatal("expected error on duplicate save")
	}
}

func TestSave_Force(t *testing.T) {
	withTempRegistry(t)

	src := makeFakeTemplate(t)
	if _, err := executeCmd("template", "save", src, "my-tag"); err != nil {
		t.Fatal(err)
	}
	if _, err := executeCmd("template", "save", "--force", src, "my-tag"); err != nil {
		t.Fatalf("template save --force: %v", err)
	}
}

func TestSave_InvalidTag(t *testing.T) {
	withTempRegistry(t)

	src := makeFakeTemplate(t)
	_, err := executeCmd("template", "save", src, "bad tag")
	if err == nil {
		t.Fatal("expected error for invalid tag")
	}
}
