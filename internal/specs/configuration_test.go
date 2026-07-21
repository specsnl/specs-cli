package specs_test

import (
	"path/filepath"
	"testing"

	"github.com/adrg/xdg"
	"github.com/specsnl/specs-cli/internal/specs"
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

func TestIsReservedName(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"name", false},
		{"_private", false},
		{"__delimiters", false}, // recognised configuration key
		{"__foo", true},
		{"__", true},
		{"__custom_config", true},
	}

	for _, tc := range cases {
		if got := specs.IsReservedName(tc.name); got != tc.want {
			t.Errorf("IsReservedName(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}
