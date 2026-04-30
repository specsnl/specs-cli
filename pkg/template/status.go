package template

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/specsnl/specs-cli/pkg/specs"
	pkggit "github.com/specsnl/specs-cli/pkg/util/git"
)

// TemplateStatus is the cached result of the most recent remote status check.
// Stored in __status.json inside each template directory.
type TemplateStatus struct {
	CheckedAt     JSONTime              `json:"CheckedAt"`
	IsUpToDate    bool                  `json:"IsUpToDate"`
	LatestVersion string                `json:"LatestVersion,omitempty"`
	ErrorKind     pkggit.CheckErrorKind `json:"ErrorKind,omitempty"`
}

// IsStale returns true when the cached status is older than 24 hours.
func (s *TemplateStatus) IsStale() bool {
	return time.Since(s.CheckedAt.Time) > 24*time.Hour
}

// LoadStatus reads __status.json from templateRoot.
// Missing file is not an error — returns nil, nil.
func LoadStatus(templateRoot string) (*TemplateStatus, error) {
	data, err := os.ReadFile(filepath.Join(templateRoot, specs.StatusFile))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var s TemplateStatus
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// SaveStatus writes __status.json into templateRoot.
func SaveStatus(templateRoot string, s *TemplateStatus) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(templateRoot, specs.StatusFile), data, 0644)
}
