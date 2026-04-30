package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/specsnl/specs-cli/pkg/specs"
)

// executeCmd creates a fresh App and root command, executes it with the given
// args, and returns captured stdout+stderr and the error.
func executeCmd(args ...string) (string, error) {
	_, out, err := executeCmdWithApp(args...)
	return out, err
}

// executeCmdWithApp is like executeCmd but also returns the App so tests can
// inspect state set by PersistentPreRunE (e.g. HookEnvPrefix).
func executeCmdWithApp(args ...string) (*App, string, error) {
	app := NewApp()
	cmd := newRootCmd(app)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return app, buf.String(), err
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

func TestHookEnvPrefix_Default(t *testing.T) {
	app, _, err := executeCmdWithApp("version")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if app.HookEnvPrefix != specs.HookEnvPrefix {
		t.Errorf("HookEnvPrefix = %q, want %q", app.HookEnvPrefix, specs.HookEnvPrefix)
	}
}

func TestHookEnvPrefix_Disabled(t *testing.T) {
	app, _, err := executeCmdWithApp("--no-env-prefix", "version")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if app.HookEnvPrefix != "" {
		t.Errorf("HookEnvPrefix = %q, want empty string", app.HookEnvPrefix)
	}
}
