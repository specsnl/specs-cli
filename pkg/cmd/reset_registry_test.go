package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/specsnl/specs-cli/pkg/specs"
)

func TestResetRegistry_WipesAndRecreates(t *testing.T) {
	withTempRegistry(t)

	// Place a sentinel file inside the registry via a save
	src := makeFakeTemplate(t)
	if _, err := executeCmd("template", "save", src, "my-tpl"); err != nil {
		t.Fatal(err)
	}

	if _, err := executeCmd("reset-registry"); err != nil {
		t.Fatalf("reset-registry: %v", err)
	}

	// Registry dir must still exist
	if _, err := os.Stat(specs.TemplateDir()); err != nil {
		t.Errorf("expected registry dir to exist after reset: %v", err)
	}
	// Saved template must be gone
	if _, err := os.Stat(filepath.Join(specs.TemplateDir(), "my-tpl")); !os.IsNotExist(err) {
		t.Error("expected my-tpl to be wiped by reset-registry")
	}
}

func TestResetRegistry_HiddenFromHelp(t *testing.T) {
	app := NewApp()
	root := newRootCmd(app)
	cmd, _, err := root.Find([]string{"reset-registry"})
	if err != nil || cmd == nil || cmd.Name() != "reset-registry" {
		t.Fatal("reset-registry command not found")
	}
	if !cmd.Hidden {
		t.Error("expected reset-registry to be hidden")
	}
}
