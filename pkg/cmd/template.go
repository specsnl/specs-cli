package cmd

import "github.com/spf13/cobra"

func newTemplateCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "template",
		Short: "Manage project templates",
		Long:  "Download, save, list, use, and manage project templates.",
	}

	cmd.AddCommand(newTemplateListCmd(app))
	cmd.AddCommand(newTemplateSaveCmd(app))
	cmd.AddCommand(newTemplateDownloadCmd(app))
	cmd.AddCommand(newTemplateValidateCmd(app))
	cmd.AddCommand(newTemplateUseCmd(app))
	cmd.AddCommand(newTemplateRenameCmd(app))
	cmd.AddCommand(newTemplateDeleteCmd(app))
	cmd.AddCommand(newTemplateUpdateCmd(app))
	cmd.AddCommand(newTemplateUpgradeCmd(app))

	return cmd
}
