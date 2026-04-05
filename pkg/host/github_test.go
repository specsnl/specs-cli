package host

import (
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantURL   string
		wantBranch string
		wantLocal bool
		wantPath  string
		wantErr   bool
	}{
		{
			name:    "github shorthand",
			input:   "github:user/repo",
			wantURL: "https://github.com/user/repo.git",
		},
		{
			name:       "github shorthand with branch",
			input:      "github:user/repo:main",
			wantURL:    "https://github.com/user/repo.git",
			wantBranch: "main",
		},
		{
			name:       "github shorthand with tag",
			input:      "github:user/repo:v1.2.3",
			wantURL:    "https://github.com/user/repo.git",
			wantBranch: "v1.2.3",
		},
		{
			name:    "github shorthand missing slash",
			input:   "github:justarepo",
			wantErr: true,
		},
		{
			name:    "full HTTPS URL without .git",
			input:   "https://github.com/user/repo",
			wantURL: "https://github.com/user/repo.git",
		},
		{
			name:    "full HTTPS URL with .git",
			input:   "https://github.com/user/repo.git",
			wantURL: "https://github.com/user/repo.git",
		},
		{
			name:    "HTTP URL",
			input:   "http://example.com/user/repo",
			wantURL: "http://example.com/user/repo.git",
		},
		{
			name:      "file prefix",
			input:     "file:./my-template",
			wantLocal: true,
			wantPath:  "./my-template",
		},
		{
			name:      "file prefix absolute path",
			input:     "file:/abs/path/to/template",
			wantLocal: true,
			wantPath:  "/abs/path/to/template",
		},
		{
			name:      "relative path implicit",
			input:     "./my-template",
			wantLocal: true,
			wantPath:  "./my-template",
		},
		{
			name:      "absolute path implicit",
			input:     "/home/user/template",
			wantLocal: true,
			wantPath:  "/home/user/template",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src, err := Parse(tc.input)

			if tc.wantErr {
				if err == nil {
					t.Errorf("Parse(%q) expected error, got nil", tc.input)
				}
				return
			}

			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tc.input, err)
			}

			if src.CloneURL != tc.wantURL {
				t.Errorf("CloneURL = %q, want %q", src.CloneURL, tc.wantURL)
			}
			if src.Branch != tc.wantBranch {
				t.Errorf("Branch = %q, want %q", src.Branch, tc.wantBranch)
			}
			if src.IsLocal != tc.wantLocal {
				t.Errorf("IsLocal = %v, want %v", src.IsLocal, tc.wantLocal)
			}
			if src.LocalPath != tc.wantPath {
				t.Errorf("LocalPath = %q, want %q", src.LocalPath, tc.wantPath)
			}
		})
	}
}
