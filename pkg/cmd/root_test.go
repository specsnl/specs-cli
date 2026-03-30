package cmd

import (
	"bytes"
	"strings"
	"testing"
)

// executeRoot resets rootCmd state and executes it with the given args.
// Returns captured stdout and the execution error.
func executeRoot(args ...string) (string, error) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(args)
	err := Execute()
	return buf.String(), err
}

func TestHelpExitsZero(t *testing.T) {
	out, err := executeRoot("--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "specs") {
		t.Errorf("expected output to contain 'specs', got: %q", out)
	}
}

func TestVersionCommand(t *testing.T) {
	out, err := executeRoot("version")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "specs version") {
		t.Errorf("expected output to contain 'specs version', got: %q", out)
	}
}

func TestVersionDontPrettify(t *testing.T) {
	out, err := executeRoot("version", "--dont-prettify")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out, "specs version") {
		t.Errorf("expected output to not contain 'specs version', got: %q", out)
	}
}

func TestTemplateGroupHelp(t *testing.T) {
	out, err := executeRoot("template", "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "template") {
		t.Errorf("expected output to contain 'template', got: %q", out)
	}
}

func TestUnknownCommandError(t *testing.T) {
	_, err := executeRoot("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown command, got nil")
	}
}
