package cmd

import (
	"context"
	"io"
	"os"
	"time"

	pkgtemplate "github.com/specsnl/specs-cli/internal/template"
	pkggit "github.com/specsnl/specs-cli/internal/util/git"
	"github.com/specsnl/specs-cli/internal/util/output"
)

// App holds application-wide dependencies shared across all commands.
type App struct {
	Output        output.Writer
	SafeMode      bool
	HookEnvPrefix string // prefix for context keys injected as env vars into hooks

	// Stdin, Stdout and Stderr are the process streams as the running command
	// sees them, populated in PersistentPreRunE from cmd.InOrStdin() and friends.
	// An interactive form is drawn through these rather than through os.Stdin and
	// os.Stdout, so a test can drive a prompt with buffers it controls — and so
	// the form never lands on stdout, which carries the product.
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	// NonInteractive refuses every prompt even at a terminal, so a developer can
	// reproduce what CI will do. A missing terminal on Stdin has the same effect
	// on its own; see App.canPrompt.
	NonInteractive bool

	// checkRemoteFn is the function used by template list to query remote status.
	// Defaults to pkggit.CheckRemoteContext; tests may substitute a fake.
	checkRemoteFn func(ctx context.Context, dir, url, branch string) pkggit.RemoteCheckResult
	// checkTimeout is the per-remote timeout for each individual status check (default 10s).
	checkTimeout time.Duration
	// refreshTimeout is the maximum wall-clock time for the entire refresh phase (default 30s).
	refreshTimeout time.Duration
}

// NewApp creates an App. The silent logger it installs writes to os.Stderr
// because no command — and therefore no cmd.ErrOrStderr() — exists yet; see
// output.SetupLogger for why it is installed twice.
func NewApp() *App {
	output.SetupLogger(os.Stderr, output.FormatPretty, false)

	return &App{
		Output:         output.NewDefaultPrettyWriter(),
		Stdin:          os.Stdin,
		Stdout:         os.Stdout,
		Stderr:         os.Stderr,
		checkRemoteFn:  pkggit.CheckRemoteContext,
		checkTimeout:   10 * time.Second,
		refreshTimeout: 30 * time.Second,
	}
}

// canPrompt reports whether an interactive form may be drawn: --non-interactive
// forbids it outright, and otherwise there has to be a terminal to answer it.
//
// Stdin is the stream that decides. A job with a terminal on stderr but its
// stdin closed would otherwise block on a read nobody can answer — the hang this
// check exists to turn into an error.
func (a *App) canPrompt() bool {
	return !a.NonInteractive && output.IsTTY(a.Stdin)
}

// promptRefusal explains why prompting is off, for the error naming what could
// not be asked for.
func (a *App) promptRefusal() string {
	if a.NonInteractive {
		return "--non-interactive is set"
	}

	return "stdin is not a terminal"
}

// templateConfig translates App-level flags into a template.Config.
// pkg/template must not import pkg/cmd, so the translation lives here.
func (a *App) templateConfig() pkgtemplate.Config {
	return pkgtemplate.Config{SafeMode: a.SafeMode}
}
