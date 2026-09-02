package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/specsnl/specs-cli/internal/specs"
	"github.com/specsnl/specs-cli/internal/util/output"
)

// Execute creates the root command and runs it with a background context.
func Execute(app *App) error {
	return ExecuteContext(context.Background(), app)
}

// ExecuteContext creates the root command and runs it with the given context.
// The context is propagated to command handlers via cmd.Context().
func ExecuteContext(ctx context.Context, app *App) error {
	return newRootCmd(app).ExecuteContext(ctx)
}

func newRootCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "specs",
		Short: "General-purpose developer CLI",
		Long: `specs is a multi-purpose developer CLI.

Use "specs <command> --help" for more information about a command.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			debug, _ := cmd.Flags().GetBool("debug")
			safeMode, _ := cmd.Flags().GetBool("safe-mode")
			noEnvPrefix, _ := cmd.Flags().GetBool("no-env-prefix")
			outputFlag, _ := cmd.Flags().GetString("output")

			app.SafeMode = safeMode
			if noEnvPrefix {
				app.HookEnvPrefix = ""
			} else {
				app.HookEnvPrefix = specs.HookEnvPrefix
			}

			format := output.Format(outputFlag)

			// The writer is wired before the format is validated, counter-intuitive
			// as that reads: the rejection below is reported through app.Output, so
			// it needs somewhere to go. Anything that is not json is wired as
			// pretty and then rejected on the next line, which costs nothing.
			app.Output = output.NewPrettyWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), nil)
			if format == output.FormatJSON {
				app.Output = output.NewJSONWriter(cmd.OutOrStdout(), cmd.ErrOrStderr())
			}

			if !format.Valid() {
				return fmt.Errorf("invalid --output %q: want %q or %q",
					outputFlag, output.FormatPretty, output.FormatJSON)
			}

			// Re-install the logger now the flags are known: on the command's own
			// stderr, so a test can read what --debug wrote, and in the format
			// --output selected. Without --debug it is silent.
			output.SetupLogger(cmd.ErrOrStderr(), format, debug)

			return nil
		},
	}

	cmd.Version = Version
	cmd.SetVersionTemplate("{{.Version}}\n")

	cmd.PersistentFlags().Bool("debug", false, "Enable debug output")
	cmd.PersistentFlags().Bool("safe-mode", false, "Disable env/filesystem template functions; implies --no-hooks (override with --allow-hooks)")
	cmd.PersistentFlags().Bool("no-env-prefix", false, "Disable the SPECS_ prefix on hook environment variables")
	cmd.PersistentFlags().StringP("output", "o", string(output.FormatPretty),
		fmt.Sprintf("Output format: %q or %q", output.FormatPretty, output.FormatJSON))

	cmd.AddCommand(newResetRegistryCmd(app))
	cmd.AddCommand(newTemplateCmd(app))
	cmd.AddCommand(newUseCmd(app))
	cmd.AddCommand(newVersionCmd(app))

	return cmd
}
