package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/specsnl/specs-cli/pkg/registry"
	"github.com/specsnl/specs-cli/pkg/specs"
)

func newTemplateUpgradeCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade [name]",
		Short: "Upgrade registered templates to the latest version",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := specs.EnsureRegistry(); err != nil {
				return err
			}

			upgradeAll := len(args) == 0

			var names []string
			if upgradeAll {
				entries, err := os.ReadDir(specs.TemplateDir())
				if err != nil {
					return err
				}
				for _, e := range entries {
					if e.IsDir() {
						names = append(names, e.Name())
					}
				}
			} else {
				names = []string{args[0]}
			}

			for _, name := range names {
				res, err := registry.Upgrade(name)
				if err != nil {
					if upgradeAll {
						app.Output.Warn("template %q: %v", name, err)
						continue
					}
					return err
				}
				switch {
				case res.IsLocal:
					app.Output.Info("template %q is a local template — skipping (no remote branch)", name)
				case res.AlreadyUpToDate:
					app.Output.Info("template %q is already up-to-date", name)
				default:
					app.Output.Info("template %q upgraded", name)
				}
			}

			return nil
		},
	}
}
