package cmd

import "github.com/spf13/cobra"

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
			WithDebug(debug)(app)
			app.SafeMode = safeMode
			return nil
		},
	}

	cmd.PersistentFlags().Bool("debug", false, "Enable debug output")
	cmd.PersistentFlags().Bool("safe-mode", false, "Disable env/filesystem template functions and hooks")

	cmd.AddCommand(newTemplateCmd())
	cmd.AddCommand(newVersionCmd())

	return cmd
}
