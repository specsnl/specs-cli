package cmd

import "github.com/spf13/cobra"

var templateCmd = &cobra.Command{
	Use:   "template",
	Short: "Manage project templates",
	Long:  "Download, save, list, use, and manage project templates.",
}
