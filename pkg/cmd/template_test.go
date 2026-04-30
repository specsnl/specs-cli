package cmd

import (
	"strings"
	"testing"
)

func TestTemplateGroup_Help(t *testing.T) {
	out, err := executeCmd("template", "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "template") {
		t.Errorf("expected output to contain 'template', got: %q", out)
	}
}
