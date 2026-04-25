package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/specsnl/specs-cli/pkg/specs"
	pkgtemplate "github.com/specsnl/specs-cli/pkg/template"
	"github.com/specsnl/specs-cli/pkg/util/output"
)

func newTemplateListCmd() *cobra.Command {
	var dontPrettify bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List registered templates",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := specs.EnsureRegistry(); err != nil {
				return err
			}

			entries, err := os.ReadDir(specs.TemplateDir())
			if err != nil {
				return err
			}

			headers := []string{"Name", "Repository", "Created"}
			var rows [][]string

			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				tmplName := e.Name()
				meta, _ := loadMetadataForListing(specs.TemplatePath(tmplName))
				repo, created := "-", "-"
				if meta != nil {
					repo = meta.Repository
					created = meta.Created.String()
				}
				rows = append(rows, []string{tmplName, repo, created})
			}

			if len(rows) == 0 {
				output.Info("no templates registered — run 'specs template download' or 'specs template save'")
				return nil
			}

			if dontPrettify {
				for _, row := range rows {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", row[0], row[1], row[2])
				}
				return nil
			}

			fmt.Fprintln(cmd.OutOrStdout(), output.RenderTable(headers, rows))
			return nil
		},
	}

	cmd.Flags().BoolVar(&dontPrettify, "dont-prettify", false, "Output tab-separated plain text instead of a styled table")

	return cmd
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
