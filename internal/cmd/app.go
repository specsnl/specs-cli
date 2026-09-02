package cmd

import (
	"context"
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

	// checkRemoteFn is the function used by template list to query remote status.
	// Defaults to pkggit.CheckRemoteContext; tests may substitute a fake.
	checkRemoteFn func(ctx context.Context, dir, url, branch string) pkggit.RemoteCheckResult
	// checkTimeout is the per-remote timeout for each individual status check (default 10s).
	checkTimeout time.Duration
	// refreshTimeout is the maximum wall-clock time for the entire refresh phase (default 30s).
	refreshTimeout time.Duration
}

// NewApp creates an App.
//
// The logger it installs is silent: nothing is emitted until PersistentPreRunE
// calls output.SetupLogger again with the resolved --debug and --output flags.
// This first call exists only so that a failure before flag parsing has a
// default logger to reach, and it writes to os.Stderr because no command — and
// therefore no cmd.ErrOrStderr() — exists yet.
func NewApp() *App {
	output.SetupLogger(os.Stderr, output.FormatPretty, false)

	return &App{
		Output:         output.NewDefaultPrettyWriter(),
		checkRemoteFn:  pkggit.CheckRemoteContext,
		checkTimeout:   10 * time.Second,
		refreshTimeout: 30 * time.Second,
	}
}

// templateConfig translates App-level flags into a template.Config.
// pkg/template must not import pkg/cmd, so the translation lives here.
func (a *App) templateConfig() pkgtemplate.Config {
	return pkgtemplate.Config{SafeMode: a.SafeMode}
}
