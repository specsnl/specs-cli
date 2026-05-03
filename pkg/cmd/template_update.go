package cmd

import (
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/specsnl/specs-cli/pkg/specs"
	pkgtemplate "github.com/specsnl/specs-cli/pkg/template"
	pkggit "github.com/specsnl/specs-cli/pkg/util/git"
)

func newTemplateUpdateCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update [name]",
		Short: "Refresh the cached status of registered templates",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := specs.EnsureRegistry(); err != nil {
				return err
			}

			var names []string
			if len(args) == 1 {
				names = []string{args[0]}
			} else {
				entries, err := os.ReadDir(specs.TemplateDir())
				if err != nil {
					return err
				}
				for _, e := range entries {
					if e.IsDir() {
						names = append(names, e.Name())
					}
				}
			}

			networkErrorSeen := false
			updatesAvailable := []string{}

			for _, name := range names {
				root := specs.TemplatePath(name)
				meta, _ := loadMetadataForListing(root)
				if meta == nil || meta.Repository == "" || meta.Branch == "" {
					continue
				}

				result, _ := pkggit.CheckRemote(root, meta.Repository, meta.Branch)

				newStatus := &pkgtemplate.TemplateStatus{
					CheckedAt:     pkgtemplate.JSONTime{Time: time.Now().UTC()},
					IsUpToDate:    result.IsUpToDate,
					LatestVersion: result.LatestVersion,
					ErrorKind:     result.ErrorKind,
				}
				_ = pkgtemplate.SaveStatus(root, newStatus)

				switch result.ErrorKind {
				case pkggit.CheckErrorNetwork:
					networkErrorSeen = true
				case pkggit.CheckErrorAuth:
					app.Output.Warn("template %q: auth error", name)
				case pkggit.CheckErrorNotFound:
					app.Output.Warn("template %q: repository not found", name)
				case pkggit.CheckErrorUnknown:
					app.Output.Warn("template %q: status check failed", name)
				default:
					if !result.IsUpToDate {
						updatesAvailable = append(updatesAvailable, name)
					}
				}
			}

			if networkErrorSeen {
				app.Output.Warn("could not reach one or more remotes — status may be outdated")
			}

			if len(updatesAvailable) > 0 {
				for _, name := range updatesAvailable {
					root := specs.TemplatePath(name)
					if s, _ := pkgtemplate.LoadStatus(root); s != nil && s.LatestVersion != "" {
						app.Output.Info("template %q has an update available: %s", name, s.LatestVersion)
					} else {
						app.Output.Info("template %q has an update available", name)
					}
				}
			} else if !networkErrorSeen {
				app.Output.Info("all templates are up-to-date")
			}

			return nil
		},
	}

	return cmd
}
