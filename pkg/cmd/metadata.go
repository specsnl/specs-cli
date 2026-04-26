package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/specsnl/specs-cli/pkg/specs"
	pkgtemplate "github.com/specsnl/specs-cli/pkg/template"
)

// writeMetadata writes __metadata.json into templateRoot.
func writeMetadata(templateRoot, name, repository, commit, version string) error {
	m := pkgtemplate.Metadata{
		Name:       name,
		Repository: repository,
		Created:    pkgtemplate.JSONTime{Time: time.Now().UTC()},
		Commit:     commit,
		Version:    version,
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(templateRoot, specs.MetadataFile), data, 0644)
}
