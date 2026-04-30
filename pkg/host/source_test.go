package host_test

import (
	"testing"

	"github.com/specsnl/specs-cli/pkg/host"
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
			name:    "full https url",
			input:   "https://github.com/user/repo",
			wantURL: "https://github.com/user/repo",
		},
		{
			name:    "https url with .git suffix",
			input:   "https://github.com/user/repo.git",
			wantURL: "https://github.com/user/repo",
		},
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
		{
			name:    "unknown format",
			input:   "foo:bar/baz",
			wantErr: true,
		},
		{
			name:    "github shorthand missing slash",
			input:   "github:repo-only",
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
