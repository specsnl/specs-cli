package cmd

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/specsnl/specs-cli/internal/specs"
	"github.com/specsnl/specs-cli/internal/util/output"
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

// executeCmdStreams keeps the two streams apart, so a test can assert which one
// a command's product and its narration land on.
func executeCmdStreams(args ...string) (stdout, stderr string, err error) {
	app := NewApp()
	cmd := newRootCmd(app)
	out, errOut := new(bytes.Buffer), new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs(args)
	err = cmd.Execute()

	return out.String(), errOut.String(), err
}

// executeCmdStreamsWith runs the command and then calls emit, which stands in
// for a log point somewhere in the tree. It runs after Execute rather than
// during it because the logger PersistentPreRunE installed is process-wide and
// still in place — so whatever emit writes lands wherever a real log point
// would have, without needing a command that happens to log.
func executeCmdStreamsWith(emit func(), args ...string) (stdout, stderr string, err error) {
	app := NewApp()
	cmd := newRootCmd(app)
	out, errOut := new(bytes.Buffer), new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs(args)
	err = cmd.Execute()

	emit()

	return out.String(), errOut.String(), err
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

func TestOutputFlag_SelectsWriter(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want output.Writer
	}{
		{"default is pretty", []string{"version"}, (*output.PrettyWriter)(nil)},
		{"explicit pretty", []string{"-o", "pretty", "version"}, (*output.PrettyWriter)(nil)},
		{"json", []string{"-o", "json", "version"}, (*output.JSONWriter)(nil)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, _, err := executeCmdWithApp(tt.args...)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got, want := fmt.Sprintf("%T", app.Output), fmt.Sprintf("%T", tt.want); got != want {
				t.Errorf("Output = %s, want %s", got, want)
			}
		})
	}
}

// An unrecognised format is rejected rather than silently treated as pretty:
// a typo in a pipeline should fail at the flag, not several lines later at jq.
func TestOutputFlag_InvalidIsRejected(t *testing.T) {
	app, _, err := executeCmdWithApp("-o", "josn", "version")
	if err == nil {
		t.Fatal("expected an error for an unknown --output value, got nil")
	}

	const want = `invalid --output "josn": want "pretty" or "json"`
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err, want)
	}

	// The writer is wired even for an invalid format, because main reports the
	// rejection through it.
	if app.Output == nil {
		t.Error("app.Output is nil; the rejection would have nowhere to go")
	}
}

// The logger PersistentPreRunE installs writes to the command's own stderr, so
// a test can read what --debug produced instead of it escaping to os.Stderr.
func TestDebugFlag_Logging(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		assert func(t *testing.T, stderr string)
	}{
		{
			name: "silent without --debug",
			args: []string{"version"},
			assert: func(t *testing.T, stderr string) {
				t.Helper()

				if strings.Contains(stderr, "level=") || strings.Contains(stderr, `"level":"DEBUG"`) {
					t.Errorf("stderr = %q, want no log records without --debug", stderr)
				}
			},
		},
		{
			name: "--debug emits text records",
			args: []string{"--debug", "version"},
			assert: func(t *testing.T, stderr string) {
				t.Helper()

				if !strings.Contains(stderr, "level=DEBUG") {
					t.Errorf("stderr = %q, want a text record", stderr)
				}
			},
		},
		{
			name: "--debug -o json emits JSON records",
			args: []string{"--debug", "-o", "json", "version"},
			assert: func(t *testing.T, stderr string) {
				t.Helper()

				if !strings.Contains(stderr, `"level":"DEBUG"`) {
					t.Errorf("stderr = %q, want a JSON record", stderr)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previous := slog.Default()

			t.Cleanup(func() { slog.SetDefault(previous) })

			_, stderr, err := executeCmdStreamsWith(func() { slog.Debug("a diagnostic") }, tt.args...)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			tt.assert(t, stderr)
		})
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
