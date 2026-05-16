package host_test

import (
	"testing"

	"github.com/specsnl/specs-cli/internal/host"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantURL    string
		wantBranch string
		wantLocal  string
		wantErr    bool
	}{
		// --- github shorthand: valid ---
		{
			name:    "github shorthand",
			input:   "github:user/repo",
			wantURL: "https://github.com/user/repo",
		},
		{
			name:       "github shorthand with branch",
			input:      "github:user/repo:main",
			wantURL:    "https://github.com/user/repo",
			wantBranch: "main",
		},
		{
			name:    "github shorthand with hyphens and underscores",
			input:   "github:foo-bar/baz_qux",
			wantURL: "https://github.com/foo-bar/baz_qux",
		},
		{
			name:       "github shorthand branch semver tag",
			input:      "github:foo/bar:v1.0.0",
			wantURL:    "https://github.com/foo/bar",
			wantBranch: "v1.0.0",
		},
		{
			name:       "github shorthand branch with slash",
			input:      "github:foo/bar:feature/x",
			wantURL:    "https://github.com/foo/bar",
			wantBranch: "feature/x",
		},
		{
			name:    "github shorthand single-char owner and repo",
			input:   "github:a/b",
			wantURL: "https://github.com/a/b",
		},

		// --- github shorthand: invalid ---
		{
			name:    "github shorthand missing slash",
			input:   "github:repo-only",
			wantErr: true,
		},
		{
			name:    "github shorthand empty repo",
			input:   "github:foo/",
			wantErr: true,
		},
		{
			name:    "github shorthand empty owner",
			input:   "github:/bar",
			wantErr: true,
		},
		{
			name:    "github shorthand owner with space",
			input:   "github:foo bar/baz",
			wantErr: true,
		},
		{
			name:    "github shorthand too many path segments",
			input:   "github:foo/bar/baz",
			wantErr: true,
		},
		{
			name:    "github shorthand empty branch after colon",
			input:   "github:foo/bar:",
			wantErr: true,
		},
		{
			name:    "github shorthand branch with dotdot",
			input:   "github:foo/bar:..",
			wantErr: true,
		},
		{
			name:    "github shorthand branch with whitespace",
			input:   "github:foo/bar:main branch",
			wantErr: true,
		},

		// --- HTTPS: valid ---
		{
			name:    "full https url",
			input:   "https://github.com/user/repo",
			wantURL: "https://github.com/user/repo",
		},
		{
			name:    "https url with .git suffix",
			input:   "https://github.com/user/repo.git",
			wantURL: "https://github.com/user/repo",
		},

		// --- HTTPS: invalid ---
		{
			name:    "https url with only one path segment",
			input:   "https://github.com/user",
			wantErr: true,
		},
		{
			name:    "https url with empty repo segment",
			input:   "https://github.com/user/",
			wantErr: true,
		},

		// --- SSH: valid ---
		{
			name:    "scp-style ssh url",
			input:   "git@github.com:user/repo",
			wantURL: "git@github.com:user/repo",
		},
		{
			name:    "scp-style ssh url with .git suffix",
			input:   "git@github.com:user/repo.git",
			wantURL: "git@github.com:user/repo",
		},
		{
			name:    "explicit ssh scheme",
			input:   "ssh://git@github.com/user/repo",
			wantURL: "ssh://git@github.com/user/repo",
		},

		// --- SSH: invalid ---
		{
			name:    "scp-style ssh url with only one path segment",
			input:   "git@github.com:user",
			wantErr: true,
		},
		{
			name:    "explicit ssh url with only one path segment",
			input:   "ssh://git@github.com/user",
			wantErr: true,
		},
		{
			name:    "explicit ssh url with empty repo segment",
			input:   "ssh://git@github.com/user/",
			wantErr: true,
		},

		// --- local paths ---
		{
			name:      "file prefix local path",
			input:     "file:./my-template",
			wantLocal: "./my-template",
		},
		{
			name:      "relative local path",
			input:     "./my-template",
			wantLocal: "./my-template",
		},
		{
			name:      "parent relative path",
			input:     "../my-template",
			wantLocal: "../my-template",
		},
		{
			name:      "absolute local path",
			input:     "/home/user/templates/my-template",
			wantLocal: "/home/user/templates/my-template",
		},

		// --- unrecognised ---
		{
			name:    "unknown format",
			input:   "foo:bar/baz",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, err := host.Parse(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Parse(%q) = nil error, want error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.input, err)
			}
			if src.CloneURL != tt.wantURL {
				t.Errorf("CloneURL = %q, want %q", src.CloneURL, tt.wantURL)
			}
			if src.Branch != tt.wantBranch {
				t.Errorf("Branch = %q, want %q", src.Branch, tt.wantBranch)
			}
			if src.LocalPath != tt.wantLocal {
				t.Errorf("LocalPath = %q, want %q", src.LocalPath, tt.wantLocal)
			}
		})
	}
}

func TestSource_IsLocal(t *testing.T) {
	local, _ := host.Parse("./my-template")
	if !local.IsLocal() {
		t.Error("./my-template should be local")
	}

	remote, _ := host.Parse("github:user/repo")
	if remote.IsLocal() {
		t.Error("github:user/repo should not be local")
	}
}

func TestSource_IsSSH(t *testing.T) {
	ssh1, _ := host.Parse("git@github.com:user/repo")
	if !ssh1.IsSSH() {
		t.Error("git@github.com:user/repo should be SSH")
	}

	ssh2, _ := host.Parse("ssh://git@github.com/user/repo")
	if !ssh2.IsSSH() {
		t.Error("ssh:// URL should be SSH")
	}

	https, _ := host.Parse("https://github.com/user/repo")
	if https.IsSSH() {
		t.Error("HTTPS URL should not be SSH")
	}
}
