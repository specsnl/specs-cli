package cmd

import (
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/specsnl/specs-cli/pkg/specs"
	"github.com/specsnl/specs-cli/pkg/util/output"
)

// Execute creates the root command and runs it.
func Execute(app *App) error {
	return newRootCmd(app).Execute()
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

			// Wire the output writer based on the --output flag.
			switch outputFlag {
			case "json":
				app.Output = output.NewJSONWriter(cmd.OutOrStdout(), cmd.ErrOrStderr())
			default:
				app.Output = output.NewHumanWriter(cmd.OutOrStdout(), cmd.ErrOrStderr())
			}

			// Configure the slog logger level; swap to JSON handler when both
			// --debug and --output=json are set.
			if debug {
				app.level.Set(slog.LevelDebug)
				if outputFlag == "json" {
					app.Logger = slog.New(slog.NewJSONHandler(cmd.ErrOrStderr(), &slog.HandlerOptions{Level: app.level}))
				}
			} else {
				app.level.Set(slog.LevelInfo)
			}

			return nil
		},
	}

	cmd.Version = Version
	cmd.SetVersionTemplate("{{.Version}}\n")

	cmd.PersistentFlags().Bool("debug", false, "Enable debug output")
	cmd.PersistentFlags().Bool("safe-mode", false, "Disable env/filesystem template functions and hooks")
	cmd.PersistentFlags().Bool("no-env-prefix", false, "Disable the SPECS_ prefix on hook environment variables")
	cmd.PersistentFlags().StringP("output", "o", "pretty", `Output format: "pretty" or "json"`)

	cmd.AddCommand(newResetRegistryCmd(app))
	cmd.AddCommand(newTemplateCmd(app))
	cmd.AddCommand(newUseCmd(app))
	cmd.AddCommand(newVersionCmd(app))

	return cmd
}
