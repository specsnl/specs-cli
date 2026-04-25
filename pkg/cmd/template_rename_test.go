package cmd

import (
	"os"
	"testing"

	"github.com/specsnl/specs-cli/pkg/specs"
)

func TestRename_Success(t *testing.T) {
	withTempRegistry(t)

	src := makeFakeTemplate(t)
	if _, err := executeCmd("template", "save", src, "old-tpl"); err != nil {
		t.Fatal(err)
	}

	if _, err := executeCmd("template", "rename", "old-tpl", "new-tpl"); err != nil {
		t.Fatalf("template rename: %v", err)
	}

	if _, err := os.Stat(specs.TemplatePath("new-tpl")); err != nil {
		t.Error("expected new-tpl to exist")
	}
	if _, err := os.Stat(specs.TemplatePath("old-tpl")); !os.IsNotExist(err) {
		t.Error("expected old-tpl to be gone")
	}
}

func TestRename_NotFound(t *testing.T) {
	withTempRegistry(t)

	_, err := executeCmd("template", "rename", "nonexistent", "new-tpl")
	if err == nil {
		t.Fatal("expected error renaming nonexistent template")
	}
}
