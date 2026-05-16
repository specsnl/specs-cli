package cmd

import (
	"errors"
	"os"
	"testing"

	"github.com/specsnl/specs-cli/internal/specs"
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

func TestRename_MvAlias(t *testing.T) {
	withTempRegistry(t)

	src := makeFakeTemplate(t)
	if _, err := executeCmd("template", "save", src, "old-tpl"); err != nil {
		t.Fatal(err)
	}
	if _, err := executeCmd("template", "mv", "old-tpl", "new-tpl"); err != nil {
		t.Fatalf("template mv: %v", err)
	}
	if _, err := os.Stat(specs.TemplatePath("new-tpl")); err != nil {
		t.Error("expected new-tpl to exist after mv")
	}
}

func TestRename_NotFound(t *testing.T) {
	withTempRegistry(t)

	_, err := executeCmd("template", "rename", "nonexistent", "new-tpl")
	if err == nil {
		t.Fatal("expected error renaming nonexistent template")
	}
}

func TestRename_NotFound_IsErrTemplateNotFound(t *testing.T) {
	withTempRegistry(t)

	_, err := executeCmd("template", "rename", "nonexistent", "new-tpl")
	if !errors.Is(err, specs.ErrTemplateNotFound) {
		t.Errorf("expected ErrTemplateNotFound, got %v", err)
	}
}

func TestRename_NameConflict_IsErrTemplateAlreadyExists(t *testing.T) {
	withTempRegistry(t)

	src := makeFakeTemplate(t)
	if _, err := executeCmd("template", "save", src, "old-tpl"); err != nil {
		t.Fatal(err)
	}
	if _, err := executeCmd("template", "save", src, "new-tpl"); err != nil {
		t.Fatal(err)
	}

	_, err := executeCmd("template", "rename", "old-tpl", "new-tpl")
	if !errors.Is(err, specs.ErrTemplateAlreadyExists) {
		t.Errorf("expected ErrTemplateAlreadyExists, got %v", err)
	}
}
