package cmd

import (
	"context"
	"log/slog"
	"os"
	"time"

	pkgtemplate "github.com/specsnl/specs-cli/pkg/template"
	pkggit "github.com/specsnl/specs-cli/pkg/util/git"
	"github.com/specsnl/specs-cli/pkg/util/output"
)

// HandlerFactory creates a slog.Handler wired to the given LevelVar.
// The LevelVar is passed so that WithDebug can adjust the level at runtime
// regardless of which handler is in use.
type HandlerFactory func(level *slog.LevelVar) slog.Handler

// Option is a functional option for configuring an App.
type Option func(*App)

// WithDebug returns an Option that sets the log level to debug when enabled,
// or back to info when false.
func WithDebug(enabled bool) Option {
	return func(a *App) {
		if enabled {
			a.level.Set(slog.LevelDebug)
		} else {
			a.level.Set(slog.LevelInfo)
		}
	}
}

// WithHandler returns an Option that replaces the global default logger with one
// built by the provided factory. The factory receives the App's LevelVar so the
// handler can honour runtime level changes from WithDebug.
//
// Example — switch to JSON output:
//
//	app := NewApp(WithHandler(func(level *slog.LevelVar) slog.Handler {
//	    return slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
//	}))
func WithHandler(factory HandlerFactory) Option {
	return func(a *App) {
		slog.SetDefault(slog.New(factory(a.level)))
	}
}

// App holds application-wide dependencies shared across all commands.
type App struct {
	Output        output.Writer
	level         *slog.LevelVar
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

// NewApp creates an App. The default logger writes text to stderr at info level and
// is registered as the global slog default. Use WithHandler to substitute a different
// handler; use WithDebug to raise the level.
// Options are applied in order after the default logger is initialised.
func NewApp(opts ...Option) *App {
	level := new(slog.LevelVar)
	level.Set(slog.LevelInfo)

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	app := &App{
		Output:         output.NewDefaultHumanWriter(),
		level:          level,
		checkRemoteFn:  pkggit.CheckRemoteContext,
		checkTimeout:   10 * time.Second,
		refreshTimeout: 30 * time.Second,
	}

	for _, opt := range opts {
		opt(app)
	}

	return app
}

// templateConfig translates App-level flags into a template.Config.
// pkg/template must not import pkg/cmd, so the translation lives here.
func (a *App) templateConfig() pkgtemplate.Config {
	return pkgtemplate.Config{SafeMode: a.SafeMode}
}
