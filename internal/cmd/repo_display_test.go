package cmd

import (
	"path/filepath"
	"testing"
)

// legacyLocalPrefix mirrors the marker older metadata carries; the production
// constant lives in internal/template and is unexported there.
const legacyLocalPrefix = "local:"

// repoCell decides three things per value — the label, the machine value and
// the link target — so every case names all three. The value is never allowed
// to change: it is what --output json emits and what the registry clones from.
func TestRepoCell(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	tests := []struct {
		name     string
		repo     string
		wantText string
		wantLink string
	}{
		{
			// GitHub is the default host, so its name is noise in the label.
			name:     "github remote loses scheme and host",
			repo:     "https://github.com/specsnl/specs-cli",
			wantText: "specsnl/specs-cli",
			wantLink: "https://github.com/specsnl/specs-cli",
		},
		{
			// Any other host keeps its name, so two templates from different
			// hosts never collapse to the same label.
			name:     "other host keeps its name",
			repo:     "https://gitlab.com/acme/platform/go-service",
			wantText: "gitlab.com/acme/platform/go-service",
			wantLink: "https://gitlab.com/acme/platform/go-service",
		},
		{
			// A port is a real part of a web address, so it stays.
			name:     "port is kept",
			repo:     "https://git.example.com:8443/owner/repo",
			wantText: "git.example.com:8443/owner/repo",
			wantLink: "https://git.example.com:8443/owner/repo",
		},
		{
			// Only the host itself is dropped, never one that merely ends in it.
			name:     "host ending in github.com is not the default host",
			repo:     "https://git.github.com/owner/repo",
			wantText: "git.github.com/owner/repo",
			wantLink: "https://git.github.com/owner/repo",
		},
		{
			name:     "http is shortened too",
			repo:     "http://github.com/owner/repo",
			wantText: "owner/repo",
			wantLink: "http://github.com/owner/repo",
		},
		{
			// More than two path segments are accepted by host.Parse, so the
			// label must not assume exactly owner/repo.
			name:     "deep github path keeps every segment",
			repo:     "https://github.com/a/b/c",
			wantText: "a/b/c",
			wantLink: "https://github.com/a/b/c",
		},
		{
			// host.Parse strips .git on download, but a hand-written
			// __metadata.json can carry it, and a trailing slash survives its
			// validation. Both are trimmed from the label only.
			name:     "trailing slash trimmed from the label",
			repo:     "https://github.com/user/repo/",
			wantText: "user/repo",
			wantLink: "https://github.com/user/repo/",
		},
		{
			name:     "dot-git trimmed from the label",
			repo:     "https://github.com/user/repo.git",
			wantText: "user/repo",
			wantLink: "https://github.com/user/repo.git",
		},
		{
			// An SSH remote is not something a terminal can open, so it is shown
			// as stored and not linked.
			name:     "scp-style ssh remote is verbatim and unlinked",
			repo:     "git@github.com:user/repo",
			wantText: "git@github.com:user/repo",
		},
		{
			name:     "ssh url is verbatim and unlinked",
			repo:     "ssh://git@git.example.com:2222/owner/repo",
			wantText: "ssh://git@git.example.com:2222/owner/repo",
		},
		{
			// The form template save now stores: already ~-collapsed, no marker.
			name:     "stored tilde path is shown as stored",
			repo:     "~/code/proto-template",
			wantText: "~/code/proto-template",
		},
		{
			name:     "stored absolute path outside home is kept whole",
			repo:     "/opt/shared/templates/go",
			wantText: "/opt/shared/templates/go",
		},
		{
			// A legacy value normalises to the same label as the new form, so
			// an install predating the change reads identically.
			name:     "legacy prefixed path under home collapses to tilde",
			repo:     legacyLocalPrefix + filepath.Join(home, "code", "proto-template"),
			wantText: "~" + string(filepath.Separator) + filepath.Join("code", "proto-template"),
		},
		{
			name:     "legacy prefixed path that is exactly home",
			repo:     legacyLocalPrefix + home,
			wantText: "~",
		},
		{
			name:     "legacy prefixed path outside home is kept whole",
			repo:     legacyLocalPrefix + "/opt/shared/templates/go",
			wantText: "/opt/shared/templates/go",
		},
		{
			// A sibling directory whose name merely starts with $HOME must not
			// be reported as living inside it.
			name:     "sibling of home is not inside it",
			repo:     legacyLocalPrefix + home + "-backup/tpl",
			wantText: home + "-backup/tpl",
		},
		{
			name:     "relative stored path",
			repo:     "./templates/go",
			wantText: "./templates/go",
		},
		{
			// The placeholder a missing __metadata.json produces arrives already
			// formed and must pass through untouched.
			name:     "placeholder passes through",
			repo:     "-",
			wantText: "-",
		},
		{
			// Metadata present with a blank field — only reachable from a
			// hand-edited or older file. Stays an empty cell.
			name:     "empty stays empty",
			repo:     "",
			wantText: "",
		},
		{
			// A legacy hand-written shorthand is not a URL and is left alone.
			name:     "legacy bare owner/repo is verbatim",
			repo:     "user/repo",
			wantText: "user/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := repoCell(tt.repo)

			if got.Value != tt.repo {
				t.Errorf("Value = %q, want the stored value %q unchanged", got.Value, tt.repo)
			}

			if got.Label() != tt.wantText {
				t.Errorf("Label() = %q, want %q", got.Label(), tt.wantText)
			}

			if got.Link != tt.wantLink {
				t.Errorf("Link = %q, want %q", got.Link, tt.wantLink)
			}
		})
	}
}

// The label is never longer than the value. A rule that lengthened it would
// defeat the purpose, and one that emptied a non-empty value would hide a row.
func TestRepoCell_LabelNeverGrows(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	repos := []string{
		"https://github.com/specsnl/specs-cli",
		"https://gitlab.com/acme/platform/go-service",
		"git@github.com:user/repo",
		"~/code/proto-template",
		legacyLocalPrefix + "/opt/shared/templates/go",
		"user/repo",
		"-",
	}

	for _, repo := range repos {
		label := repoCell(repo).Label()
		if len(label) > len(repo) {
			t.Errorf("repoCell(%q) label %q is longer than the value", repo, label)
		}

		if label == "" && repo != "" {
			t.Errorf("repoCell(%q) produced an empty label", repo)
		}
	}
}

// A remote is linked and a path is not, which is what decides whether a row is
// clickable at all.
func TestRepoCell_OnlyRemotesAreLinked(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	for _, local := range []string{"~/x", "/opt/x", "./x", legacyLocalPrefix + "/opt/x"} {
		if cell := repoCell(local); cell.Link != "" {
			t.Errorf("local path %q was linked to %q", local, cell.Link)
		}
	}

	if cell := repoCell("https://github.com/a/b"); cell.Link == "" {
		t.Error("a remote was not linked")
	}
}
