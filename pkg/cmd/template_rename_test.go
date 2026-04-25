package cmd

import (
	"os"
	"testing"

	"github.com/specsnl/specs-cli/pkg/specs"
)

func TestRename_Success(t *testing.T) {
	withTempRegistry(t)

	src := makeFakeTemplate(t)
	if _, err := executeCmd("template", "save", src, "old-tag"); err != nil {
		t.Fatal(err)
	}

	if _, err := executeCmd("template", "rename", "old-tag", "new-tag"); err != nil {
		t.Fatalf("template rename: %v", err)
	}

	if _, err := os.Stat(specs.TemplatePath("new-tag")); err != nil {
		t.Error("expected new-tag to exist")
	}
	if _, err := os.Stat(specs.TemplatePath("old-tag")); !os.IsNotExist(err) {
		t.Error("expected old-tag to be gone")
	}
}

func TestRename_NotFound(t *testing.T) {
	withTempRegistry(t)

	_, err := executeCmd("template", "rename", "nonexistent", "new-tag")
	if err == nil {
		t.Fatal("expected error renaming nonexistent template")
	}
}
