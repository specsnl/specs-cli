package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/specsnl/specs-cli/pkg/specs"
	pkgtemplate "github.com/specsnl/specs-cli/pkg/template"
	pkggit "github.com/specsnl/specs-cli/pkg/util/git"
)

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
				meta, _ := loadMetadataForListing(root)
				var status *pkgtemplate.TemplateStatus
				if meta != nil && meta.Repository != "" && meta.Branch != "" {
					status, _ = pkgtemplate.LoadStatus(root)
				}
				tmplEntries = append(tmplEntries, templateEntry{name: name, meta: meta, status: status})
			}

			// Refresh stale statuses in parallel.
			var mu sync.Mutex
			var wg sync.WaitGroup
			networkErrorSeen := false

			for i, entry := range tmplEntries {
				if entry.meta == nil || entry.meta.Repository == "" || entry.meta.Branch == "" {
					continue
				}
				if entry.status != nil && !entry.status.IsStale() {
					continue
				}
				wg.Add(1)
				go func(i int, name, repo, branch string) {
					defer wg.Done()
					root := specs.TemplatePath(name)
					result, _ := pkggit.CheckRemote(root, repo, branch)
					newStatus := &pkgtemplate.TemplateStatus{
						CheckedAt:     pkgtemplate.JSONTime{Time: time.Now().UTC()},
						IsUpToDate:    result.IsUpToDate,
						LatestVersion: result.LatestVersion,
						ErrorKind:     result.ErrorKind,
					}
					_ = pkgtemplate.SaveStatus(root, newStatus)
					mu.Lock()
					tmplEntries[i].status = newStatus
					if result.ErrorKind == pkggit.CheckErrorNetwork {
						networkErrorSeen = true
					}
					mu.Unlock()
				}(i, entry.name, entry.meta.Repository, entry.meta.Branch)
			}
			wg.Wait()

			headers := []string{"Name", "Repository", "Version", "Status", "Created"}
			var rows [][]string

			for _, entry := range tmplEntries {
				repo, version, created := "-", "-", "-"
				if entry.meta != nil {
					repo = entry.meta.Repository
					created = entry.meta.Created.String()
					if entry.meta.Version != "" {
						version = entry.meta.Version
					}
				}
				hasRemote := entry.meta != nil && entry.meta.Repository != "" && entry.meta.Branch != ""
				statusStr := statusLabel(entry.status, hasRemote)
				rows = append(rows, []string{entry.name, repo, version, statusStr, created})
			}

			if len(rows) == 0 {
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

// statusLabel returns the Status column string for a template.
func statusLabel(status *pkgtemplate.TemplateStatus, hasRemote bool) string {
	if !hasRemote {
		return "-"
	}
	if status == nil {
		return "unknown"
	}
	switch status.ErrorKind {
	case pkggit.CheckErrorNetwork:
		return "unknown (offline?)"
	case pkggit.CheckErrorAuth:
		return "auth error"
	case pkggit.CheckErrorNotFound:
		return "not found"
	case pkggit.CheckErrorUnknown:
		return "check failed"
	}
	if status.IsUpToDate {
		return "up-to-date"
	}
	if status.LatestVersion != "" {
		return "update: " + status.LatestVersion
	}
	return "update available"
}

func loadMetadataForListing(templateRoot string) (*pkgtemplate.Metadata, error) {
	path := filepath.Join(templateRoot, specs.MetadataFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil //nolint:nilerr // missing metadata is not an error
	}
	var m pkgtemplate.Metadata
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, nil //nolint:nilerr // malformed metadata is silently ignored when listing
	}
	return &m, nil
}
