package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pkgtemplate "github.com/specsnl/specs-cli/internal/template"
)

func TestUpdate_NoArgs_EmptyRegistry(t *testing.T) {
	withTempRegistry(t)

	_, err := executeCmd("template", "update")
	if err != nil {
		t.Fatalf("template update with empty registry: %v", err)
	}
}

func TestUpdate_NamedLocalTemplate_Skipped(t *testing.T) {
	registryDir := withTempRegistry(t)

	// "local:" prefix is what `specs template save` stores in Repository.
	tmplDir := filepath.Join(registryDir, "local-tpl")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := pkgtemplate.SaveMetadata(tmplDir, "local-tpl", "local:/some/local/path", "", "", "", time.Now().UTC(), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	// Should succeed: local templates are silently skipped.
	_, err := executeCmd("template", "update", "local-tpl")
	if err != nil {
		t.Fatalf("template update local-tpl: %v", err)
	}
}

func TestUpdate_TooManyArgs(t *testing.T) {
	withTempRegistry(t)

	_, err := executeCmd("template", "update", "a", "b")
	if err == nil {
		t.Fatal("expected error when too many args given")
	}
}

// assertNothingChecked asserts the JSON product is an empty record set and that
// the explanation was narrated on stderr instead of mixed into it.
func assertNothingChecked(t *testing.T, stdout, stderr string) {
	t.Helper()

	if got := strings.TrimSpace(stdout); got != "[]" {
		t.Errorf("stdout = %q, want an empty record set %q", got, "[]")
	}

	if !strings.Contains(stderr, "no trackable templates") {
		t.Errorf("expected the hint on stderr, got: %q", stderr)
	}
}

func TestUpdate_NamedLocalTemplate_ReportsNothingChecked(t *testing.T) {
	registryDir := withTempRegistry(t)

	// "local:" prefix is what `specs template save` stores in Repository — silently skipped.
	tmplDir := filepath.Join(registryDir, "local-tpl")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := pkgtemplate.SaveMetadata(tmplDir, "local-tpl", "local:/some/local/path", "", "", "", time.Now().UTC(), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := executeCmdStreams("template", "update", "local-tpl", "--output=json")
	if err != nil {
		t.Fatalf("template update local-tpl: %v", err)
	}

	assertNothingChecked(t, stdout, stderr)
}

// TestUpdate_LocalTemplate_WithGitHistory_ReportsNothingChecked reproduces the bug where a template
// saved from a git-tracked directory had git history in the registry copy. CurrentBranch would
// succeed, causing CheckRemote to be called with the "local:/path" URL and triggering a DNS error.
func TestUpdate_LocalTemplate_WithGitHistory_ReportsNothingChecked(t *testing.T) {
	registryDir := withTempRegistry(t)

	tmplDir := filepath.Join(registryDir, "local-git-tpl")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Simulate a .git directory so CurrentBranch would succeed if we reached that code.
	if err := os.MkdirAll(filepath.Join(tmplDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := pkgtemplate.SaveMetadata(tmplDir, "local-git-tpl", "local:/Users/user/my-template", "", "", "", time.Now().UTC(), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := executeCmdStreams("template", "update", "local-git-tpl", "--output=json")
	if err != nil {
		t.Fatalf("template update local-git-tpl: %v", err)
	}

	// No row and no warning: the local: repository must not be checked over the network.
	assertNothingChecked(t, stdout, stderr)
}

// A checked template is a row of the answer, on stdout.
func TestUpdate_CheckedTemplate_IsARowOnStdout(t *testing.T) {
	tests := []struct {
		name    string
		advance bool
		// The version the check reports is only set once the source moved on, and
		// it is a `git describe` string, so the advanced case matches a prefix.
		wantStatus string
		wantLatest string
	}{
		{"up-to-date", false, `"Status":"up-to-date"`, `"Latest":"-"`},
		{"advanced source", true, `"Status":"update available"`, `"Latest":"1.0.0-1-g`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registryDir := withTempRegistry(t)
			src, repo, _ := makeLocalTemplate(t, registryDir, "loc")

			if tt.advance {
				localCommit(t, repo, src, "second")
			}

			stdout, stderr, err := executeCmdStreams("template", "update", "--output=json")
			if err != nil {
				t.Fatalf("template update: %v", err)
			}

			got := strings.TrimSpace(stdout)
			for _, want := range []string{`"Name":"loc"`, tt.wantStatus, tt.wantLatest} {
				if !strings.Contains(got, want) {
					t.Errorf("stdout = %q, want it to contain %q", got, want)
				}
			}

			if stderr != "" {
				t.Errorf("expected no narration, got: %q", stderr)
			}
		})
	}
}

func TestUpdate_NoArgs_EmptyRegistry_ReportsNothingChecked(t *testing.T) {
	withTempRegistry(t)

	stdout, stderr, err := executeCmdStreams("template", "update", "--output=json")
	if err != nil {
		t.Fatalf("template update with empty registry: %v", err)
	}

	assertNothingChecked(t, stdout, stderr)
}
