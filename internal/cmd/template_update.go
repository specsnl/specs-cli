package cmd

import (
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/specsnl/specs-cli/internal/specs"
	pkgtemplate "github.com/specsnl/specs-cli/internal/template"
	pkggit "github.com/specsnl/specs-cli/internal/util/git"
	"github.com/spf13/cobra"
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
			checkedCount := 0

			var rows [][]string

			for _, name := range names {
				root := specs.TemplatePath(name)

				meta, err := pkgtemplate.LoadMetadata(root)
				if err != nil {
					slog.Debug("failed to parse template metadata", "template", name, "error", err)
				}

				if meta == nil || meta.Repository == "" {
					continue
				}

				var result pkggit.RemoteCheckResult

				if isLocalRepo(meta.Repository) {
					if meta.Commit == "" {
						// Nothing recorded to compare the source path against.
						continue
					}

					checkedCount++
					// git layer logs the check-local start/result
					result = pkggit.CheckLocalSource(strings.TrimPrefix(meta.Repository, localRepoPrefix), meta.Commit, meta.Version)
				} else {
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
					result = pkggit.CheckRemote(root, meta.Repository, branch)
				}

				newStatus := &pkgtemplate.TemplateStatus{
					CheckedAt:     pkgtemplate.JSONTime{Time: time.Now().UTC()},
					IsUpToDate:    result.IsUpToDate,
					LatestVersion: result.LatestVersion,
					ErrorKind:     result.ErrorKind,
					SpecsVersion:  Version,
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
				case pkggit.CheckErrorSourceMissing:
					app.Output.Warn("template %q: local source path is missing", name)
				case pkggit.CheckErrorUnknown:
					app.Output.Warn("template %q: status check failed", name)
				}

				rows = append(rows, []string{name, updateStatusLabel(result), latestOrDash(result.LatestVersion)})
			}

			if networkErrorSeen {
				app.Output.Warn("could not reach one or more remotes — status may be outdated")
			}

			// The table is the answer, empty or not; the hint that explains an
			// empty one is narration.
			app.Output.Table([]string{"Name", "Status", "Latest"}, rows)

			if checkedCount == 0 {
				app.Output.Info("no trackable templates — nothing to check")
			}

			return nil
		},
	}

	return cmd
}

// updateStatusLabel renders the Status column for one just-checked template.
// The version, when there is one, belongs in the Latest column rather than
// inside this string — unlike `template list`, which reads a cached status.
func updateStatusLabel(result pkggit.RemoteCheckResult) string {
	if label := checkErrorLabel(result.ErrorKind); label != "" {
		return label
	}

	if result.IsUpToDate {
		return "up-to-date"
	}

	return "update available"
}

func latestOrDash(version string) string {
	if version == "" {
		return "-"
	}

	return version
}
