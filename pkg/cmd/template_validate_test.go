package cmd

import (
	"testing"
)

func TestValidate_ValidTemplate(t *testing.T) {
	withTempRegistry(t)
	src := makeFakeTemplate(t)

	if _, err := executeCmd("template", "validate", src); err != nil {
		t.Fatalf("template validate: %v", err)
	}
}

func TestValidate_MissingTemplateDir(t *testing.T) {
	withTempRegistry(t)

	// A directory without a template/ subdirectory
	src := t.TempDir()
	_, err := executeCmd("template", "validate", src)
	if err == nil {
		t.Fatal("expected error for missing template/ subdir")
	}
}
