package template_test

import (
	"path/filepath"
	"testing"

	pkgtemplate "github.com/specsnl/specs-cli/internal/template"
)

// The legacy marker, as it still appears in every __metadata.json written
// before it was dropped. The production constant is unexported.
const legacyLocalPrefix = "local:"

// A path is told from a remote by its first character. Getting this wrong in
// either direction is serious: a remote treated as a path is never cloned, and
// a path treated as a remote is handed to git.
func TestIsLocalRepository(t *testing.T) {
	local := []string{
		"/opt/templates/go",
		"/Users/me/code/tpl",
		"~/code/tpl",
		"~",
		"./templates/go",
		"../templates/go",
		".",
		legacyLocalPrefix + "/opt/templates/go",
		legacyLocalPrefix + "~/code/tpl",
	}

	remote := []string{
		"https://github.com/specsnl/specs-cli",
		"http://example.com/a/b",
		"git@github.com:user/repo",
		"ssh://git@github.com/user/repo",
		"user/repo",
		"",
	}

	for _, repo := range local {
		if !pkgtemplate.IsLocalRepository(repo) {
			t.Errorf("IsLocalRepository(%q) = false, want true", repo)
		}
	}

	for _, repo := range remote {
		if pkgtemplate.IsLocalRepository(repo) {
			t.Errorf("IsLocalRepository(%q) = true, want false", repo)
		}
	}
}

// LocalSourcePath is what reaches the filesystem, so it must always come back
// as something a syscall understands — never a literal ~ and never a marker.
func TestLocalSourcePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	tests := []struct {
		name string
		repo string
		want string
	}{
		{
			name: "tilde is expanded",
			repo: "~/code/tpl",
			want: filepath.Join(home, "code", "tpl"),
		},
		{
			name: "bare tilde is the home directory",
			repo: "~",
			want: home,
		},
		{
			name: "absolute path is unchanged",
			repo: "/opt/templates/go",
			want: "/opt/templates/go",
		},
		{
			name: "relative path is unchanged",
			repo: "./templates/go",
			want: "./templates/go",
		},
		{
			name: "legacy prefix is stripped",
			repo: legacyLocalPrefix + "/opt/templates/go",
			want: "/opt/templates/go",
		},
		{
			name: "legacy prefix and tilde together",
			repo: legacyLocalPrefix + "~/code/tpl",
			want: filepath.Join(home, "code", "tpl"),
		},
		{
			// A name merely starting with ~ is not home-relative.
			name: "tilde-prefixed name is not expanded",
			repo: "~notahome/tpl",
			want: "~notahome/tpl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pkgtemplate.LocalSourcePath(tt.repo); got != tt.want {
				t.Errorf("LocalSourcePath(%q) = %q, want %q", tt.repo, got, tt.want)
			}
		})
	}
}

func TestCollapseHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "under home",
			path: filepath.Join(home, "code", "tpl"),
			want: "~" + string(filepath.Separator) + filepath.Join("code", "tpl"),
		},
		{
			name: "exactly home",
			path: home,
			want: "~",
		},
		{
			name: "outside home",
			path: "/opt/templates/go",
			want: "/opt/templates/go",
		},
		{
			// A sibling whose name starts with $HOME is not inside it.
			name: "sibling of home",
			path: home + "-backup/tpl",
			want: home + "-backup/tpl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pkgtemplate.CollapseHome(tt.path); got != tt.want {
				t.Errorf("CollapseHome(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

// What save stores must read back as the path it was given, whichever side of
// $HOME it lives on. A break here silently repoints every saved template.
func TestStorableSourcePath_RoundTrips(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	paths := []string{
		filepath.Join(home, "code", "tpl"),
		filepath.Join(home, "a", "b", "c"),
		home,
		"/opt/shared/templates/go",
		home + "-backup/tpl",
	}

	for _, abs := range paths {
		stored := pkgtemplate.StorableSourcePath(abs)

		if !pkgtemplate.IsLocalRepository(stored) {
			t.Errorf("StorableSourcePath(%q) = %q, which is not recognised as local", abs, stored)
		}

		if got := pkgtemplate.LocalSourcePath(stored); got != abs {
			t.Errorf("%q stored as %q read back as %q", abs, stored, got)
		}
	}
}

// A value written by an older version resolves to the same path as the new form
// of the same directory. This is the assertion that an existing install keeps
// working, and it is the highest-risk property of the change.
func TestLegacyAndCurrentFormsAgree(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	abs := filepath.Join(home, "code", "tpl")

	legacy := legacyLocalPrefix + abs
	current := pkgtemplate.StorableSourcePath(abs)

	if legacy == current {
		t.Fatal("the two forms are identical, so this test proves nothing")
	}

	if got, want := pkgtemplate.LocalSourcePath(legacy), pkgtemplate.LocalSourcePath(current); got != want {
		t.Errorf("legacy %q resolves to %q, current %q resolves to %q", legacy, got, current, want)
	}

	if !pkgtemplate.IsLocalRepository(legacy) || !pkgtemplate.IsLocalRepository(current) {
		t.Error("both forms must be recognised as local")
	}
}
