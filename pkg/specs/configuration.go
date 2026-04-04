package specs

import (
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
)

const (
	AppName         = "specs"
	TemplateDirName = "templates"

	// File names inside a template root
	ProjectYAMLFile = "project.yaml"
	ProjectJSONFile = "project.json" // backward-compat fallback
	MetadataFile    = "__metadata.json"
	IgnoreFile      = ".specsignore"
	TemplateDirFile = "template" // the subdirectory that gets rendered
)

// ConfigDir returns the specs configuration directory.
// Defaults to $XDG_CONFIG_HOME/specs (~/.config/specs).
func ConfigDir() string {
	return filepath.Join(xdg.ConfigHome, AppName)
}

// TemplateDir returns the directory where registered templates are stored.
func TemplateDir() string {
	return filepath.Join(ConfigDir(), TemplateDirName)
}

// TemplatePath returns the full path to a specific registered template by tag.
func TemplatePath(tag string) string {
	return filepath.Join(TemplateDir(), tag)
}

// IsRegistryInitialised reports whether the template directory exists on disk.
func IsRegistryInitialised() bool {
	info, err := os.Stat(TemplateDir())
	return err == nil && info.IsDir()
}
