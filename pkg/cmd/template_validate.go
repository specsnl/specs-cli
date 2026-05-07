package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/specsnl/specs-cli/pkg/specs"
	pkgtemplate "github.com/specsnl/specs-cli/pkg/template"
	"github.com/specsnl/specs-cli/pkg/util/exit"
)

func newTemplateValidateCmd(app *App) *cobra.Command {
	var strict bool

	cmd := &cobra.Command{
		Use:   "validate <path>",
		Short: "Validate a template directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app.level.Set(slog.LevelDebug)

			templateRoot := args[0]

			templateDir := filepath.Join(templateRoot, specs.TemplateDirFile)
			if info, err := os.Stat(templateDir); err != nil || !info.IsDir() {
				return specs.ErrTemplateDirMissing
			}

			cfg := app.templateConfig()
			cfg.ContinueOnRenderError = true // validate collects all render errors; never aborts early
			tmpl, err := pkgtemplate.Get(templateRoot, cfg, app.Logger)
			if err != nil {
				return fmt.Errorf("invalid template: %w", err)
			}

			// Select fields are []any until the user picks one at prompt time.
			// Use the first option so the template engine can compare them as strings.
			resolveSelectDefaults(tmpl.Context)

			tmp, err := os.MkdirTemp("", "specs-validate-*")
			if err != nil {
				return err
			}
			defer os.RemoveAll(tmp)

			if err := tmpl.Execute(tmp); err != nil {
				return fmt.Errorf("template render error: %w", err)
			}

			for _, w := range tmpl.Warnings {
				app.Output.Warn("render error in %s: %v", w.Path, w.Err)
			}

			issues, err := tmpl.Validate()
			if err != nil {
				return fmt.Errorf("validation error: %w", err)
			}

			for _, iss := range issues {
				switch {
				case errors.Is(iss, pkgtemplate.ErrUnknownVariable):
					app.Output.Warn("variable %q used in %s is not defined in project.yaml", iss.Name, iss.File)
				case errors.Is(iss, pkgtemplate.ErrUnusedVariable):
					app.Output.Warn("variable %q is defined but never used in any template file", iss.Name)
				case errors.Is(iss, pkgtemplate.ErrUnusedComputed):
					app.Output.Warn("computed value %q is defined but never used in any template file", iss.Name)
				}
			}

			code := 0
			if len(tmpl.Warnings) > 0 {
				code |= exit.ValidateRender
			}
			if pkgtemplate.HasUnknown(issues) {
				code |= exit.ValidateUnknown
			}
			if strict && pkgtemplate.HasUnused(issues) {
				code |= exit.ValidateUnused
			}
			if code != 0 {
				return &exit.ExitError{Code: code}
			}

			app.Output.Info("template is valid")
			return nil
		},
	}

	cmd.Flags().BoolVar(&strict, "strict", false, "Treat unused variables and computed values as errors")
	return cmd
}
