package specs

import (
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
)

const (
	AppName         = "specs"
	TemplateDirName = "templates"

	// HookEnvPrefix is the default prefix for context env vars injected into hooks.
	HookEnvPrefix = "SPECS_"

	// File names inside a template root
	ProjectYAMLFile = "project.yaml"
	ProjectYMLFile  = "project.yml"
	ProjectJSONFile = "project.json" // backward-compat fallback
	MetadataFile    = "__metadata.json"
	StatusFile      = "__status.json"
	VerbatimFile    = ".specsverbatim"
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

// TemplatePath returns the full path to a specific registered template by name.
func TemplatePath(name string) string {
	return filepath.Join(TemplateDir(), name)
}

// IsRegistryInitialised reports whether the template directory exists on disk.
func IsRegistryInitialised() bool {
	info, err := os.Stat(TemplateDir())
	return err == nil && info.IsDir()
}

// EnsureRegistry creates the template registry directory if it does not already exist.
func EnsureRegistry() error {
	return os.MkdirAll(TemplateDir(), 0755)
}
