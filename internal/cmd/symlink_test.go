package cmd

import (
	"errors"
	"maps"
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/specsnl/specs-cli/internal/specs"
	"github.com/specsnl/specs-cli/internal/util/exit"
)

// buildLinkedTemplate writes a template with a file link, a directory link, a
// templated link and the given extra links into dir.
func buildLinkedTemplate(t *testing.T, dir string, extra map[string]string) {
	t.Helper()

	buildMinimalTemplate(t, dir, "Name: acme\n", ".nvmrc", "26")

	tplDir := filepath.Join(dir, specs.TemplateDirFile)
	files := map[string]string{"real/f.txt": "f", "{{ .Name }}.rc": "rc"}

	for rel, content := range files {
		path := filepath.Join(tplDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	links := map[string]string{".node-version": ".nvmrc", "alias": "real", "cfg": "{{ .Name }}.rc"}
	maps.Copy(links, extra)

	for link, target := range links {
		if err := os.Symlink(target, filepath.Join(tplDir, link)); err != nil {
			t.Fatal(err)
		}
	}
}

func assertOutputLinks(t *testing.T, dir string) {
	t.Helper()

	for link, want := range map[string]string{".node-version": ".nvmrc", "alias": "real", "cfg": "acme.rc"} {
		got, err := os.Readlink(filepath.Join(dir, link))
		if err != nil {
			t.Errorf("%s is not a link: %v", link, err)
			continue
		}

		if got != want {
			t.Errorf("%s → %q, want %q", link, got, want)
		}
	}
}

// serveGitRepo commits the template build writes and serves it over smart HTTP
// through git http-backend. It returns the URL to clone.
func serveGitRepo(t *testing.T, build func(dir string)) string {
	t.Helper()

	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}

	root := t.TempDir()
	dir := filepath.Join(root, "acme", "tpl")

	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	build(dir)

	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"add", "-A"},
		{"commit", "-q", "-m", "init"},
	} {
		cmd := exec.Command(git, append([]string{
			"-C", dir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false",
		}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	srv := httptest.NewServer(&cgi.Handler{
		Path: git,
		Args: []string{"http-backend"},
		Env:  []string{"GIT_PROJECT_ROOT=" + root, "GIT_HTTP_EXPORT_ALL=1"},
	})
	t.Cleanup(srv.Close)

	return srv.URL + "/acme/tpl.git"
}

func TestUse_LocalTemplateKeepsLinks(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}

	srcDir := t.TempDir()
	buildLinkedTemplate(t, srcDir, map[string]string{"leak.txt": outside, "broken": "nope", "up": "../project.yaml"})

	targetDir := t.TempDir()

	out, err := executeCmd("use", "--use-defaults", "file:"+srcDir, targetDir)
	if err != nil {
		t.Fatalf("use: %v\n%s", err, out)
	}

	assertOutputLinks(t, targetDir)

	for _, skipped := range []string{"leak.txt", "broken", "up"} {
		if _, err := os.Lstat(filepath.Join(targetDir, skipped)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s should not be in the output: %v", skipped, err)
		}

		if !strings.Contains(out, "template/"+skipped+" →") {
			t.Errorf("output should warn about %s:\n%s", skipped, out)
		}
	}
}

func TestTemplateUse_RegisteredTemplateKeepsLinks(t *testing.T) {
	withTempRegistry(t)

	srcDir := t.TempDir()
	buildLinkedTemplate(t, srcDir, nil)

	if out, err := executeCmd("template", "save", srcDir, "linked"); err != nil {
		t.Fatalf("save: %v\n%s", err, out)
	}

	targetDir := t.TempDir()
	if out, err := executeCmd("template", "use", "--use-defaults", "linked", targetDir); err != nil {
		t.Fatalf("template use: %v\n%s", err, out)
	}

	assertOutputLinks(t, targetDir)
}

func TestTemplateValidate_ReportsLinks(t *testing.T) {
	dir := t.TempDir()
	buildLinkedTemplate(t, dir, map[string]string{"broken": "nope", "up": "../project.yaml"})

	out, err := executeCmd("template", "validate", dir)

	if code := validateExitCode(err); code != exit.ValidateLinks {
		t.Fatalf("exit code = %d, want %d (err=%v)\n%s", code, exit.ValidateLinks, err, out)
	}

	for _, want := range []string{"template/broken → nope", "template/up → ../project.yaml"} {
		if !strings.Contains(out, want) {
			t.Errorf("output should mention %q:\n%s", want, out)
		}
	}
}

func TestTemplateValidate_InBoundsLinksAreValid(t *testing.T) {
	dir := t.TempDir()
	buildLinkedTemplate(t, dir, nil)

	if out, err := executeCmd("template", "validate", dir); err != nil {
		t.Fatalf("validate: %v\n%s", err, out)
	}
}

func TestUse_RemoteOutsideLinkAborts(t *testing.T) {
	url := serveGitRepo(t, func(dir string) {
		buildLinkedTemplate(t, dir, map[string]string{"leak": "../../../../.ssh/id_rsa"})
	})

	targetDir := filepath.Join(t.TempDir(), "out")

	_, err := executeCmd("use", "--use-defaults", url, targetDir)
	if !errors.Is(err, specs.ErrSymlinkOutside) {
		t.Fatalf("use error = %v, want ErrSymlinkOutside", err)
	}

	if !strings.Contains(err.Error(), "template/leak → ../../../../.ssh/id_rsa") {
		t.Errorf("error should name the link and its target: %v", err)
	}

	if _, err := os.Stat(targetDir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("nothing should be written to the target: %v", err)
	}
}

func TestUse_RemoteInBoundsLinksAreKept(t *testing.T) {
	url := serveGitRepo(t, func(dir string) { buildLinkedTemplate(t, dir, nil) })

	targetDir := t.TempDir()

	if out, err := executeCmd("use", "--use-defaults", url, targetDir); err != nil {
		t.Fatalf("use: %v\n%s", err, out)
	}

	assertOutputLinks(t, targetDir)
}

func TestDownload_RemoteOutsideLinkAborts(t *testing.T) {
	withTempRegistry(t)

	url := serveGitRepo(t, func(dir string) {
		buildLinkedTemplate(t, dir, map[string]string{"leak": "/etc/passwd"})
	})

	_, err := executeCmd("template", "download", url, "leaky")
	if !errors.Is(err, specs.ErrSymlinkOutside) {
		t.Fatalf("download error = %v, want ErrSymlinkOutside", err)
	}

	if _, err := os.Stat(specs.TemplatePath("leaky")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a rejected template should not be registered: %v", err)
	}
}
