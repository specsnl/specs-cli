package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/specsnl/specs-cli/pkg/specs"
	pkgtemplate "github.com/specsnl/specs-cli/pkg/template"
)

// writeMetadata writes __metadata.json into templateRoot. The created
// timestamp is supplied by the caller so that upgrades can preserve the
// original install time instead of overwriting it.
func writeMetadata(templateRoot, name, repository, branch, commit, version string, created time.Time) error {
	m := pkgtemplate.Metadata{
		Name:       name,
		Repository: repository,
		Branch:     branch,
		Created:    pkgtemplate.JSONTime{Time: created},
		Commit:     commit,
		Version:    version,
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(templateRoot, specs.MetadataFile), data, 0644)
}
