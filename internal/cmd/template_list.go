package cmd

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/specsnl/specs-cli/internal/specs"
	pkgtemplate "github.com/specsnl/specs-cli/internal/template"
	pkggit "github.com/specsnl/specs-cli/internal/util/git"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

// localRepoPrefix marks a Repository value that refers to a directory on disk
// (registered via 'specs template save') rather than a remote git URL.
const localRepoPrefix = "local:"

// isLocalRepo reports whether repo refers to a local source path.
func isLocalRepo(repo string) bool {
	return strings.HasPrefix(repo, localRepoPrefix)
}

// isTrackable reports whether a template carries enough metadata for a status
// check. A remote template needs a repository and a branch; a local template
// needs a recorded commit to compare its source path against.
func isTrackable(meta *pkgtemplate.Metadata) bool {
	if meta == nil || meta.Repository == "" {
		return false
	}

	if isLocalRepo(meta.Repository) {
		return meta.Commit != ""
	}

	return meta.Branch != ""
}

func newTemplateListCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List registered templates",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := specs.EnsureRegistry(); err != nil {
				return err
			}

			entries, err := os.ReadDir(specs.TemplateDir())
			if err != nil {
				return err
			}

			type templateEntry struct {
				name   string
				meta   *pkgtemplate.Metadata
				status *pkgtemplate.TemplateStatus
			}

			var tmplEntries []templateEntry

			for _, e := range entries {
				if !e.IsDir() {
					continue
				}

				name := e.Name()
				root := specs.TemplatePath(name)

				meta, err := pkgtemplate.LoadMetadata(root)
				if err != nil {
					slog.Debug("failed to parse template metadata", "template", name, "error", err)
				}
				// Resolve and persist the branch when metadata has none so that the
				// status refresh below and hasRemote checks work correctly.
				if meta != nil && meta.Repository != "" && meta.Branch == "" {
					if b, err := pkggit.CurrentBranch(root); err == nil {
						meta.Branch = b
						if err := pkgtemplate.SaveMetadata(root, name, meta.Repository, b, meta.Commit, meta.Version, meta.Created.Time, meta.Updated.Time); err != nil {
							slog.Debug("failed to persist resolved branch", "template", name, "error", err)
						}
					}
				}

				var status *pkgtemplate.TemplateStatus
				if meta != nil && meta.Repository != "" && meta.Branch != "" {
					status, err = pkgtemplate.LoadStatus(root)
					if err != nil {
						slog.Debug("failed to load template status", "template", name, "error", err)
					}
				}

				tmplEntries = append(tmplEntries, templateEntry{name: name, meta: meta, status: status})
			}

			// Refresh stale statuses in parallel, capped at 8 concurrent checks.
			// A top-level timeout guards the whole phase; each check also has its own timeout.
			const maxConcurrency = 8

			var mu sync.Mutex

			networkErrorSeen := false

			refreshCtx, cancelRefresh := context.WithTimeout(cmd.Context(), app.refreshTimeout)
			defer cancelRefresh()

			eg, egCtx := errgroup.WithContext(refreshCtx)
			eg.SetLimit(maxConcurrency)

			for i, entry := range tmplEntries {
				if !isTrackable(entry.meta) {
					continue
				}
				// A version change forces a refresh even within the 24h window, so a
				// status written by an older binary with different check logic is not trusted.
				if entry.status != nil && !entry.status.NeedsRefresh(Version) {
					continue
				}

				i, name := i, entry.name
				repo, branch := entry.meta.Repository, entry.meta.Branch
				commit, version := entry.meta.Commit, entry.meta.Version
				local := isLocalRepo(repo)

				eg.Go(func() error {
					root := specs.TemplatePath(name)

					var result pkggit.RemoteCheckResult
					if local {
						// Local templates compare against the source path on disk, not a
						// git remote. The git layer logs the check-local start/result.
						result = pkggit.CheckLocalSource(strings.TrimPrefix(repo, localRepoPrefix), commit, version)
					} else {
						checkCtx, cancelCheck := context.WithTimeout(egCtx, app.checkTimeout)
						defer cancelCheck()
						// git layer logs the check-remote start/result
						result = app.checkRemoteFn(checkCtx, root, repo, branch)
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

					mu.Lock()
					tmplEntries[i].status = newStatus

					if result.ErrorKind == pkggit.CheckErrorNetwork {
						networkErrorSeen = true
					}
					mu.Unlock()

					return nil
				})
			}

			_ = eg.Wait()

			headers := []string{"Name", "Repository", "Version", "Status", "Created", "Updated"}

			var rows [][]string

			for _, entry := range tmplEntries {
				repo, version, created, updated := "-", "-", "-", "-"
				if entry.meta != nil {
					repo = entry.meta.Repository
					created = entry.meta.Created.String()

					updated = entry.meta.Updated.String()
					if entry.meta.Version != "" {
						version = entry.meta.Version
					}
				}

				statusStr := statusLabel(entry.status, isTrackable(entry.meta))
				rows = append(rows, []string{entry.name, repo, version, statusStr, created, updated})
			}

			// The empty answer keeps the shape of the non-empty one — an empty
			// table on stdout — so a consumer parses one document either way.
			if len(rows) == 0 {
				app.Output.Table(headers, nil)
				app.Output.Info("no templates registered — run 'specs template download' or 'specs template save'")

				return nil
			}

			app.Output.Table(headers, rows)

			if networkErrorSeen {
				app.Output.Warn("could not reach one or more remotes — status may be outdated")
			}

			return nil
		},
	}

	return cmd
}

// statusLabel returns the Status column string for a template. tracked reports
// whether the template has a source (remote branch or local path) whose status
// can be checked; untracked templates always render as "-".
func statusLabel(status *pkgtemplate.TemplateStatus, tracked bool) string {
	if !tracked {
		return "-"
	}

	if status == nil {
		return "unknown"
	}

	if label := checkErrorLabel(status.ErrorKind); label != "" {
		return label
	}

	if status.IsUpToDate {
		return "up-to-date"
	}

	if status.LatestVersion != "" {
		return "update: " + status.LatestVersion
	}

	return "update available"
}

// checkErrorLabel renders why a status check failed, or "" when it did not.
// Shared by the Status column of `template list` and `template update`.
func checkErrorLabel(kind pkggit.CheckErrorKind) string {
	switch kind {
	case pkggit.CheckErrorNetwork:
		return "unknown (offline?)"
	case pkggit.CheckErrorAuth:
		return "auth error"
	case pkggit.CheckErrorNotFound:
		return "not found"
	case pkggit.CheckErrorSourceMissing:
		return "source missing"
	case pkggit.CheckErrorUnknown:
		return "check failed"
	default:
		return ""
	}
}
