package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/specsnl/specs-cli/pkg/specs"
	pkgtemplate "github.com/specsnl/specs-cli/pkg/template"
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
	if _, err := executeCmd("template", "save", src, "my-tpl"); err != nil {
		t.Fatalf("template save: %v", err)
	}

	if _, err := os.Stat(specs.TemplatePath("my-tpl")); err != nil {
		t.Errorf("expected registry entry to exist: %v", err)
	}
}

func TestSave_AlreadyExists(t *testing.T) {
	withTempRegistry(t)

	src := makeFakeTemplate(t)
	if _, err := executeCmd("template", "save", src, "my-tpl"); err != nil {
		t.Fatal(err)
	}

	_, err := executeCmd("template", "save", src, "my-tpl")
	if err == nil {
		t.Fatal("expected error on duplicate save")
	}
}

func TestSave_Force(t *testing.T) {
	withTempRegistry(t)

	src := makeFakeTemplate(t)
	if _, err := executeCmd("template", "save", src, "my-tpl"); err != nil {
		t.Fatal(err)
	}
	if _, err := executeCmd("template", "save", "--force", src, "my-tpl"); err != nil {
		t.Fatalf("template save --force: %v", err)
	}
}

func TestSave_InvalidName(t *testing.T) {
	withTempRegistry(t)

	src := makeFakeTemplate(t)
	_, err := executeCmd("template", "save", src, "bad name")
	if err == nil {
		t.Fatal("expected error for invalid name")
	}
}

func TestSave_StoresLocalAbsolutePath(t *testing.T) {
	withTempRegistry(t)

	src := makeFakeTemplate(t)
	if _, err := executeCmd("template", "save", src, "my-tpl"); err != nil {
		t.Fatalf("template save: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(specs.TemplatePath("my-tpl"), specs.MetadataFile))
	if err != nil {
		t.Fatalf("reading metadata: %v", err)
	}
	var meta pkgtemplate.Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("parsing metadata: %v", err)
	}

	if !strings.HasPrefix(meta.Repository, "local:") {
		t.Errorf("Repository should start with \"local:\", got %q", meta.Repository)
	}
	absPath := strings.TrimPrefix(meta.Repository, "local:")
	if !filepath.IsAbs(absPath) {
		t.Errorf("path after \"local:\" should be absolute, got %q", absPath)
	}
}
