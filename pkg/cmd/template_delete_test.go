package cmd

import (
	"os"
	"testing"

	"github.com/specsnl/specs-cli/pkg/specs"
)

func TestDelete_Success(t *testing.T) {
	withTempRegistry(t)

	src := makeFakeTemplate(t)
	if _, err := executeCmd("template", "save", src, "my-tpl"); err != nil {
		t.Fatal(err)
	}

	if _, err := executeCmd("template", "delete", "my-tpl"); err != nil {
		t.Fatalf("template delete: %v", err)
	}

	if _, err := os.Stat(specs.TemplatePath("my-tpl")); !os.IsNotExist(err) {
		t.Error("expected my-tpl to be deleted")
	}
}

func TestDelete_Aliases(t *testing.T) {
	for _, alias := range []string{"rm", "remove", "del"} {
		t.Run(alias, func(t *testing.T) {
			withTempRegistry(t)
			src := makeFakeTemplate(t)
			if _, err := executeCmd("template", "save", src, "my-tpl"); err != nil {
				t.Fatal(err)
			}
			if _, err := executeCmd("template", alias, "my-tpl"); err != nil {
				t.Fatalf("template %s: %v", alias, err)
			}
			if _, err := os.Stat(specs.TemplatePath("my-tpl")); !os.IsNotExist(err) {
				t.Errorf("expected my-tpl to be deleted via %q", alias)
			}
		})
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
	if _, err := executeCmd("template", "save", src, "tpl-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := executeCmd("template", "save", src, "tpl-b"); err != nil {
		t.Fatal(err)
	}

	if _, err := executeCmd("template", "delete", "tpl-a", "tpl-b"); err != nil {
		t.Fatalf("template delete multiple: %v", err)
	}

	for _, name := range []string{"tpl-a", "tpl-b"} {
		if _, err := os.Stat(specs.TemplatePath(name)); !os.IsNotExist(err) {
			t.Errorf("expected %q to be deleted", name)
		}
	}
}
