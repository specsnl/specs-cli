package cmd

import "github.com/spf13/cobra"

func newTemplateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "template",
		Short: "Manage project templates",
		Long:  "Download, save, list, use, and manage project templates.",
	}
	return cmd
}
