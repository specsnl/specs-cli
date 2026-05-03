package cmd

import (
	"log/slog"
	"os"

	pkgtemplate "github.com/specsnl/specs-cli/pkg/template"
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

// WithHandler returns an Option that replaces the logger with one built by the
// provided factory. The factory receives the App's LevelVar so the handler can
// honour runtime level changes from WithDebug.
//
// Example — switch to JSON output:
//
//	app := NewApp(WithHandler(func(level *slog.LevelVar) slog.Handler {
//	    return slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
//	}))
func WithHandler(factory HandlerFactory) Option {
	return func(a *App) {
		a.Logger = slog.New(factory(a.level))
	}
}

// App holds application-wide dependencies shared across all commands.
type App struct {
	Logger        *slog.Logger
	Output        output.Writer
	level         *slog.LevelVar
	SafeMode      bool
	HookEnvPrefix string // prefix for context keys injected as env vars into hooks
}

// NewApp creates an App. The default logger writes text to stderr at info level.
// Use WithHandler to substitute a different handler; use WithDebug to raise the level.
// Options are applied in order after the default logger is initialised.
func NewApp(opts ...Option) *App {
	level := new(slog.LevelVar)
	level.Set(slog.LevelInfo)

	app := &App{
		Logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})),
		Output: output.NewDefaultHumanWriter(),
		level:  level,
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

// templateGet is a convenience wrapper that calls template.Get with the App's
// logger, so callers don't have to pass it explicitly every time.
func (a *App) templateGet(templateRoot string) (*pkgtemplate.Template, error) {
	return pkgtemplate.Get(templateRoot, a.templateConfig(), a.Logger)
}
