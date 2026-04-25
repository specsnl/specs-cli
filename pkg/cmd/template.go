package cmd

import "github.com/spf13/cobra"

func newTemplateCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "template",
		Short: "Manage project templates",
		Long:  "Download, save, list, use, and manage project templates.",
	}

	cmd.AddCommand(newTemplateListCmd())
	cmd.AddCommand(newTemplateSaveCmd())
	cmd.AddCommand(newTemplateDownloadCmd())
	cmd.AddCommand(newTemplateValidateCmd(app))
	cmd.AddCommand(newTemplateUseCmd(app))
	cmd.AddCommand(newTemplateRenameCmd())
	cmd.AddCommand(newTemplateDeleteCmd())

	return cmd
}
