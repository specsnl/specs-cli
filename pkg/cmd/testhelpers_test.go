package cmd

import (
	"testing"

	"github.com/adrg/xdg"
	"github.com/specsnl/specs-cli/pkg/specs"
)

// withTempRegistry sets XDG_CONFIG_HOME to a temp directory and reloads xdg,
// returning the path to the template registry directory.
func withTempRegistry(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	xdg.Reload()
	t.Cleanup(func() { xdg.Reload() })
	return specs.TemplateDir()
}
