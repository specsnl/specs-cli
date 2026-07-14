package cmd

import (
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/specsnl/specs-cli/internal/specs"
	pkgtemplate "github.com/specsnl/specs-cli/internal/template"
	pkggit "github.com/specsnl/specs-cli/internal/util/git"
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
			checkedCount := 0

			for _, name := range names {
				root := specs.TemplatePath(name)
				meta, err := pkgtemplate.LoadMetadata(root)
				if err != nil {
					slog.Debug("failed to parse template metadata", "template", name, "error", err)
				}
				if meta == nil || meta.Repository == "" || strings.HasPrefix(meta.Repository, "local:") {
					continue
				}

				branch := meta.Branch
				if branch == "" {
					b, err := pkggit.CurrentBranch(root)
					if err != nil {
						slog.Debug("could not resolve branch from local HEAD, skipping", "template", name, "error", err)
						continue
					}
					branch = b
					// Persist the resolved branch so future runs skip this fallback.
					if err := pkgtemplate.SaveMetadata(root, name, meta.Repository, branch, meta.Commit, meta.Version, meta.Created.Time, meta.Updated.Time); err != nil {
						slog.Debug("failed to persist resolved branch", "template", name, "error", err)
					}
				}

				checkedCount++
				// git layer logs the check-remote start/result
				result := pkggit.CheckRemote(root, meta.Repository, branch)

				newStatus := &pkgtemplate.TemplateStatus{
					CheckedAt:     pkgtemplate.JSONTime{Time: time.Now().UTC()},
					IsUpToDate:    result.IsUpToDate,
					LatestVersion: result.LatestVersion,
					ErrorKind:     result.ErrorKind,
				}
				if err := pkgtemplate.SaveStatus(root, newStatus); err != nil {
					slog.Debug("failed to save template status", "template", name, "error", err)
				}

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
					s, err := pkgtemplate.LoadStatus(root)
					if err != nil {
						slog.Debug("failed to load template status", "template", name, "error", err)
					}
					if s != nil && s.LatestVersion != "" {
						app.Output.Info("template %q has an update available: %s", name, s.LatestVersion)
					} else {
						app.Output.Info("template %q has an update available", name)
					}
				}
			} else if !networkErrorSeen && checkedCount > 0 {
				if len(names) == 1 {
					app.Output.Info("template %q is up-to-date", names[0])
				} else {
					app.Output.Info("all templates are up-to-date")
				}
			}

			return nil
		},
	}

	return cmd
}
