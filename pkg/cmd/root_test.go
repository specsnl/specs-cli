package cmd

import (
	"bytes"
	"strings"
	"testing"
)

// executeCmd creates a fresh App and root command, executes it with the given
// args, and returns captured stdout+stderr and the error.
func executeCmd(args ...string) (string, error) {
	app := NewApp()
	cmd := newRootCmd(app)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestHelp_ExitsZero(t *testing.T) {
	out, err := executeCmd("--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "specs") {
		t.Errorf("expected output to contain 'specs', got: %q", out)
	}
}

func TestUnknownCommand_ReturnsError(t *testing.T) {
	_, err := executeCmd("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown command, got nil")
	}
}
