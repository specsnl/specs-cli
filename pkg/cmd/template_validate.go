package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/specsnl/specs-cli/pkg/specs"
	"github.com/specsnl/specs-cli/pkg/util/output"
)

func newTemplateValidateCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "validate <path>",
		Short: "Validate a template directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			templateRoot := args[0]

			templateDir := filepath.Join(templateRoot, specs.TemplateDirFile)
			if info, err := os.Stat(templateDir); err != nil || !info.IsDir() {
				return specs.ErrTemplateDirMissing
			}

			tmpl, err := app.templateGet(templateRoot)
			if err != nil {
				return fmt.Errorf("invalid template: %w", err)
			}

			tmp, err := os.MkdirTemp("", "specs-validate-*")
			if err != nil {
				return err
			}
			defer os.RemoveAll(tmp)

			if err := tmpl.Execute(tmp); err != nil {
				return fmt.Errorf("template render error: %w", err)
			}

			output.Info("template is valid")
			return nil
		},
	}
}
