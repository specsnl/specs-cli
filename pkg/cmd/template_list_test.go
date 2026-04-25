package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestList_LsAlias(t *testing.T) {
	withTempRegistry(t)

	if _, err := executeCmd("template", "ls"); err != nil {
		t.Fatalf("template ls: %v", err)
	}
}

func TestList_Empty(t *testing.T) {
	withTempRegistry(t)

	if _, err := executeCmd("template", "list"); err != nil {
		t.Fatalf("template list: %v", err)
	}
}

func TestList_ShowsTemplate(t *testing.T) {
	registryDir := withTempRegistry(t)

	// Manually place a template directory
	if err := os.MkdirAll(filepath.Join(registryDir, "my-tpl"), 0755); err != nil {
		t.Fatal(err)
	}

	out, err := executeCmd("template", "list")
	if err != nil {
		t.Fatalf("template list: %v", err)
	}
	if !strings.Contains(out, "my-tpl") {
		t.Errorf("expected output to contain 'my-tpl', got: %q", out)
	}
}

func TestList_DontPrettify(t *testing.T) {
	registryDir := withTempRegistry(t)
	if err := os.MkdirAll(filepath.Join(registryDir, "my-tpl"), 0755); err != nil {
		t.Fatal(err)
	}

	out, err := executeCmd("template", "list", "--dont-prettify")
	if err != nil {
		t.Fatalf("template list --dont-prettify: %v", err)
	}
	if !strings.Contains(out, "\t") {
		t.Errorf("expected tab-separated output, got: %q", out)
	}
}
