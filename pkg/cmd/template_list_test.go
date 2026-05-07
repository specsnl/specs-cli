package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	pkgtemplate "github.com/specsnl/specs-cli/pkg/template"
	pkggit "github.com/specsnl/specs-cli/pkg/util/git"
)

// executeCmdWithCheckFn runs the command with a custom checkRemoteFn injected into the App.
func executeCmdWithCheckFn(
	fn func(ctx context.Context, dir, url, branch string) (pkggit.RemoteCheckResult, error),
	args ...string,
) (string, error) {
	app := NewApp()
	app.checkRemoteFn = fn
	app.checkTimeout = 200 * time.Millisecond
	app.refreshTimeout = 5 * time.Second
	cmd := newRootCmd(app)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

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
	if err := pkgtemplate.SaveMetadata(tmplDir, "local-tpl", "/local/path", "", "", "", time.Now().UTC()); err != nil {
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
	if err := pkgtemplate.SaveMetadata(tmplDir, "remote-tpl", "https://example.com/repo", "main", "", "", time.Now().UTC()); err != nil {
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
	if err := pkgtemplate.SaveMetadata(tmplDir, "remote-tpl", "https://example.com/repo", "main", "", "", time.Now().UTC()); err != nil {
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

func TestList_ConcurrencyCap(t *testing.T) {
	const numTemplates = 20
	registryDir := withTempRegistry(t)

	// Create numTemplates templates with stale statuses so all trigger a refresh.
	for i := range numTemplates {
		name := fmt.Sprintf("tpl-%02d", i)
		tmplDir := filepath.Join(registryDir, name)
		if err := os.MkdirAll(tmplDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := pkgtemplate.SaveMetadata(tmplDir, name, "https://example.com/repo", "main", "", "", time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
		stale := &pkgtemplate.TemplateStatus{
			CheckedAt: pkgtemplate.JSONTime{Time: time.Now().Add(-25 * time.Hour)},
			ErrorKind: pkggit.CheckErrorNetwork,
		}
		if err := pkgtemplate.SaveStatus(tmplDir, stale); err != nil {
			t.Fatal(err)
		}
	}

	var (
		mu      sync.Mutex
		current int
		peak    int
	)

	fake := func(ctx context.Context, dir, url, branch string) (pkggit.RemoteCheckResult, error) {
		mu.Lock()
		current++
		if current > peak {
			peak = current
		}
		mu.Unlock()
		// Simulate a brief network round-trip.
		select {
		case <-time.After(20 * time.Millisecond):
		case <-ctx.Done():
		}
		mu.Lock()
		current--
		mu.Unlock()
		return pkggit.RemoteCheckResult{IsUpToDate: true}, nil
	}

	if _, err := executeCmdWithCheckFn(fake, "template", "list"); err != nil {
		t.Fatalf("template list: %v", err)
	}

	if peak > 8 {
		t.Errorf("peak concurrent CheckRemote calls = %d, want ≤ 8", peak)
	}
}

func TestList_PerCheckTimeout(t *testing.T) {
	registryDir := withTempRegistry(t)
	tmplDir := filepath.Join(registryDir, "slow-tpl")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := pkgtemplate.SaveMetadata(tmplDir, "slow-tpl", "https://example.com/repo", "main", "", "", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	stale := &pkgtemplate.TemplateStatus{
		CheckedAt: pkgtemplate.JSONTime{Time: time.Now().Add(-25 * time.Hour)},
		ErrorKind: pkggit.CheckErrorNetwork,
	}
	if err := pkgtemplate.SaveStatus(tmplDir, stale); err != nil {
		t.Fatal(err)
	}

	var called atomic.Bool
	fake := func(ctx context.Context, dir, url, branch string) (pkggit.RemoteCheckResult, error) {
		called.Store(true)
		// Block until the per-check context times out.
		<-ctx.Done()
		return pkggit.RemoteCheckResult{ErrorKind: pkggit.CheckErrorNetwork}, nil
	}

	start := time.Now()
	out, err := executeCmdWithCheckFn(fake, "template", "list")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("template list: %v", err)
	}
	if !called.Load() {
		t.Fatal("expected fake checkRemoteFn to be called")
	}
	// The per-check timeout is 200ms in executeCmdWithCheckFn; the command must finish well before 2s.
	if elapsed > 2*time.Second {
		t.Errorf("command took %v, expected to finish within 2s after check timeout", elapsed)
	}
	// The result should still render (list doesn't fail on network errors).
	if !strings.Contains(out, "slow-tpl") {
		t.Errorf("expected output to contain 'slow-tpl', got: %q", out)
	}
}
