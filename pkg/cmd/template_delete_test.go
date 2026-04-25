package cmd

import (
	"os"
	"testing"

	"github.com/specsnl/specs-cli/pkg/specs"
)

func TestDelete_Success(t *testing.T) {
	withTempRegistry(t)

	src := makeFakeTemplate(t)
	if _, err := executeCmd("template", "save", src, "my-tag"); err != nil {
		t.Fatal(err)
	}

	if _, err := executeCmd("template", "delete", "my-tag"); err != nil {
		t.Fatalf("template delete: %v", err)
	}

	if _, err := os.Stat(specs.TemplatePath("my-tag")); !os.IsNotExist(err) {
		t.Error("expected my-tag to be deleted")
	}
}

func TestDelete_NotFound(t *testing.T) {
	withTempRegistry(t)

	_, err := executeCmd("template", "delete", "nonexistent")
	if err == nil {
		t.Fatal("expected error deleting nonexistent template")
	}
}

func TestDelete_MultipleArgs(t *testing.T) {
	withTempRegistry(t)

	src := makeFakeTemplate(t)
	if _, err := executeCmd("template", "save", src, "tag-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := executeCmd("template", "save", src, "tag-b"); err != nil {
		t.Fatal(err)
	}

	if _, err := executeCmd("template", "delete", "tag-a", "tag-b"); err != nil {
		t.Fatalf("template delete multiple: %v", err)
	}

	for _, tag := range []string{"tag-a", "tag-b"} {
		if _, err := os.Stat(specs.TemplatePath(tag)); !os.IsNotExist(err) {
			t.Errorf("expected %q to be deleted", tag)
		}
	}
}
