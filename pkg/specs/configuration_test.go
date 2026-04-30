package specs_test

import (
	"path/filepath"
	"testing"

	"github.com/adrg/xdg"
	"github.com/specsnl/specs-cli/pkg/specs"
)

func TestConfigDir_XDGOverride(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	xdg.Reload()
	t.Cleanup(func() { xdg.Reload() })

	got := specs.ConfigDir()
	want := filepath.Join(tmp, "specs")
	if got != want {
		t.Errorf("ConfigDir() = %q, want %q", got, want)
	}
}

func TestTemplateDir_XDGOverride(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	xdg.Reload()
	t.Cleanup(func() { xdg.Reload() })

	got := specs.TemplateDir()
	want := filepath.Join(tmp, "specs", "templates")
	if got != want {
		t.Errorf("TemplateDir() = %q, want %q", got, want)
	}
}

