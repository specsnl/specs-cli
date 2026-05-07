package template

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/specsnl/specs-cli/pkg/specs"
)

// Metadata is stored in __metadata.json inside each registered template.
type Metadata struct {
	Name       string   `json:"Name"`
	Repository string   `json:"Repository"`
	Branch     string   `json:"Branch,omitempty"`
	Created    JSONTime `json:"Created"`
	Commit     string   `json:"Commit,omitempty"`
	Version    string   `json:"Version,omitempty"`
}

// JSONTime wraps time.Time with RFC1123Z serialisation and a human-readable display.
type JSONTime struct {
	time.Time
}

func (t JSONTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Time.Format(time.RFC1123Z))
}

func (t *JSONTime) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	parsed, err := time.Parse(time.RFC1123Z, s)
	if err != nil {
		return err
	}
	t.Time = parsed
	return nil
}

// String returns a human-readable relative time string ("3 days ago").
func (t JSONTime) String() string {
	d := time.Since(t.Time)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d minutes ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	}
}

// LoadMetadata reads __metadata.json from templateRoot.
// Missing or malformed metadata is not an error — returns nil, nil.
func LoadMetadata(templateRoot string) (*Metadata, error) {
	path := filepath.Join(templateRoot, specs.MetadataFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil //nolint:nilerr // missing or unreadable metadata is not an error
	}
	var m Metadata
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, nil //nolint:nilerr // malformed metadata is silently ignored
	}
	return &m, nil
}

// SaveMetadata writes __metadata.json into templateRoot. The created timestamp is
// supplied by the caller so that upgrades can preserve the original install time.
func SaveMetadata(templateRoot, name, repository, branch, commit, version string, created time.Time) error {
	m := Metadata{
		Name:       name,
		Repository: repository,
		Branch:     branch,
		Created:    JSONTime{Time: created},
		Commit:     commit,
		Version:    version,
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(templateRoot, specs.MetadataFile), data, 0644)
}
