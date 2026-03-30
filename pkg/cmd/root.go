package cmd

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:   "specs",
	Short: "General-purpose developer CLI",
	Long: `specs is a multi-purpose developer CLI.

Use "specs <command> --help" for more information about a command.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(templateCmd)
	rootCmd.AddCommand(versionCmd)
}
