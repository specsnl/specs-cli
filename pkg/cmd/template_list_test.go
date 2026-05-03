package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pkgtemplate "github.com/specsnl/specs-cli/pkg/template"
	pkggit "github.com/specsnl/specs-cli/pkg/util/git"
)

func TestList_LsAlias(t *testing.T) {
	withTempRegistry(t)

	if _, err := executeCmd("template", "ls"); err != nil {
		t.Fatalf("template ls: %v", err)
	}
}

func TestList_Empty(t *testing.T) {
	withTempRegistry(t)

	if _, err := executeCmd("template", "list"); err != nil {
		t.Fatalf("template list: %v", err)
	}
}

func TestList_ShowsTemplate(t *testing.T) {
	registryDir := withTempRegistry(t)

	// Manually place a template directory
	if err := os.MkdirAll(filepath.Join(registryDir, "my-tpl"), 0755); err != nil {
		t.Fatal(err)
	}

	out, err := executeCmd("template", "list")
	if err != nil {
		t.Fatalf("template list: %v", err)
	}
	if !strings.Contains(out, "my-tpl") {
		t.Errorf("expected output to contain 'my-tpl', got: %q", out)
	}
}

func TestList_JSONOutput(t *testing.T) {
	registryDir := withTempRegistry(t)
	if err := os.MkdirAll(filepath.Join(registryDir, "my-tpl"), 0755); err != nil {
		t.Fatal(err)
	}

	out, err := executeCmd("template", "list", "--output=json")
	if err != nil {
		t.Fatalf("template list --output=json: %v", err)
	}
	if !strings.Contains(out, `"Name"`) {
		t.Errorf("expected JSON with Name key, got: %q", out)
	}
	if !strings.Contains(out, "my-tpl") {
		t.Errorf("expected JSON to contain 'my-tpl', got: %q", out)
	}
}

func TestList_StatusColumn_LocalNoStatus(t *testing.T) {
	registryDir := withTempRegistry(t)
	tmplDir := filepath.Join(registryDir, "local-tpl")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Local template: repository set but no branch.
	if err := writeMetadata(tmplDir, "local-tpl", "/local/path", "", "", ""); err != nil {
		t.Fatal(err)
	}

	out, err := executeCmd("template", "list", "--output=json")
	if err != nil {
		t.Fatalf("template list: %v", err)
	}
	if !strings.Contains(out, `"Status":"-"`) {
		t.Errorf("expected '-' status for local template, got: %q", out)
	}
}

func TestList_StatusColumn_FreshUpToDate(t *testing.T) {
	registryDir := withTempRegistry(t)
	tmplDir := filepath.Join(registryDir, "remote-tpl")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := writeMetadata(tmplDir, "remote-tpl", "https://example.com/repo", "main", "", ""); err != nil {
		t.Fatal(err)
	}
	// Write a fresh status so no network call is made.
	status := &pkgtemplate.TemplateStatus{
		CheckedAt:  pkgtemplate.JSONTime{Time: time.Now()},
		IsUpToDate: true,
	}
	if err := pkgtemplate.SaveStatus(tmplDir, status); err != nil {
		t.Fatal(err)
	}

	out, err := executeCmd("template", "list", "--output=json")
	if err != nil {
		t.Fatalf("template list: %v", err)
	}
	if !strings.Contains(out, "up-to-date") {
		t.Errorf("expected 'up-to-date' in output, got: %q", out)
	}
}

func TestStatusLabel(t *testing.T) {
	tests := []struct {
		name      string
		status    *pkgtemplate.TemplateStatus
		hasRemote bool
		want      string
	}{
		{"no remote", nil, false, "-"},
		{"nil status with remote", nil, true, "unknown"},
		{"network error", &pkgtemplate.TemplateStatus{ErrorKind: pkggit.CheckErrorNetwork}, true, "unknown (offline?)"},
		{"auth error", &pkgtemplate.TemplateStatus{ErrorKind: pkggit.CheckErrorAuth}, true, "auth error"},
		{"not found", &pkgtemplate.TemplateStatus{ErrorKind: pkggit.CheckErrorNotFound}, true, "not found"},
		{"unknown error", &pkgtemplate.TemplateStatus{ErrorKind: pkggit.CheckErrorUnknown}, true, "check failed"},
		{"up-to-date", &pkgtemplate.TemplateStatus{IsUpToDate: true}, true, "up-to-date"},
		{"update with version", &pkgtemplate.TemplateStatus{LatestVersion: "v2.0.0"}, true, "update: v2.0.0"},
		{"update available", &pkgtemplate.TemplateStatus{}, true, "update available"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := statusLabel(tc.status, tc.hasRemote)
			if got != tc.want {
				t.Errorf("statusLabel(%+v, %v) = %q, want %q", tc.status, tc.hasRemote, got, tc.want)
			}
		})
	}
}

func TestList_StatusColumn_NetworkWarn(t *testing.T) {
	registryDir := withTempRegistry(t)
	tmplDir := filepath.Join(registryDir, "remote-tpl")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := writeMetadata(tmplDir, "remote-tpl", "https://example.com/repo", "main", "", ""); err != nil {
		t.Fatal(err)
	}
	// Write a stale status with network error.
	status := &pkgtemplate.TemplateStatus{
		CheckedAt: pkgtemplate.JSONTime{Time: time.Now().Add(-25 * time.Hour)},
		ErrorKind: pkggit.CheckErrorNetwork,
	}
	if err := pkgtemplate.SaveStatus(tmplDir, status); err != nil {
		t.Fatal(err)
	}

	// The status is stale so CheckRemote will be called. Since the repo dir has
	// no git repo, it returns CheckErrorUnknown (not network). But statusLabel
	// with a fresh network-error status shows "unknown (offline?)".
	// We verify that the stale-refresh path doesn't panic and the command succeeds.
	_, err := executeCmd("template", "list")
	if err != nil {
		t.Fatalf("template list: %v", err)
	}
}
