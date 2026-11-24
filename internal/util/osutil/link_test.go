package osutil_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/specsnl/specs-cli/internal/specs"
	"github.com/specsnl/specs-cli/internal/util/osutil"
)

func mustSymlink(t *testing.T, target, link string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(link), 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink %s → %s: %v", link, target, err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// newLinkTree returns a root holding a tree of links inside a parent that also
// has a file outside the root.
func newLinkTree(t *testing.T) (parent, root string) {
	t.Helper()

	parent = t.TempDir()
	root = filepath.Join(parent, "root")

	mustWrite(t, filepath.Join(parent, "outside.txt"), "secret")
	mustWrite(t, filepath.Join(root, ".nvmrc"), "26")
	mustWrite(t, filepath.Join(root, "real", "f.txt"), "f")
	mustWrite(t, filepath.Join(root, "real", "sub", "g.txt"), "g")

	return parent, root
}

func TestResolveLink(t *testing.T) {
	tests := []struct {
		name    string
		links   map[string]string // link (relative to root) → target
		check   string
		want    string // relative to root; empty when an error is expected
		wantErr error
	}{
		{name: "file", links: map[string]string{".node-version": ".nvmrc"}, check: ".node-version", want: ".nvmrc"},
		{name: "directory", links: map[string]string{"alias": "real"}, check: "alias", want: "real"},
		{name: "nested parent", links: map[string]string{"real/sub/up": "../f.txt"}, check: "real/sub/up", want: "real/f.txt"},
		{name: "chain", links: map[string]string{"a": "b", "b": "real/f.txt"}, check: "a", want: "real/f.txt"},
		{name: "through a directory link", links: map[string]string{"alias": "real", "g": "alias/sub/g.txt"}, check: "g", want: "real/sub/g.txt"},
		{name: "absolute", links: map[string]string{"leak": "/etc/passwd"}, check: "leak", wantErr: specs.ErrSymlinkOutside},
		{name: "relative escape", links: map[string]string{"leak": "../outside.txt"}, check: "leak", wantErr: specs.ErrSymlinkOutside},
		{name: "chain escapes", links: map[string]string{"a": "b", "b": "../outside.txt"}, check: "a", wantErr: specs.ErrSymlinkOutside},
		{name: "leaves and comes back", links: map[string]string{"a": "../root/.nvmrc"}, check: "a", wantErr: specs.ErrSymlinkOutside},
		// Lexically real/sub/.. is real, but the OS climbs from sub's target.
		{name: "dot-dot after a link", links: map[string]string{"real/esc": "..", "a": "real/esc/../outside.txt"}, check: "a", wantErr: specs.ErrSymlinkOutside},
		{name: "dangling", links: map[string]string{"broken": "nope"}, check: "broken", wantErr: specs.ErrSymlinkDangling},
		{name: "loop", links: map[string]string{"a": "b", "b": "a"}, check: "a", wantErr: specs.ErrSymlinkDangling},
		{name: "through a file", links: map[string]string{"a": ".nvmrc/x"}, check: "a", wantErr: specs.ErrSymlinkDangling},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, root := newLinkTree(t)

			for link, target := range tt.links {
				mustSymlink(t, target, filepath.Join(root, filepath.FromSlash(link)))
			}

			got, err := osutil.ResolveLink(root, filepath.Join(root, filepath.FromSlash(tt.check)))

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ResolveLink error = %v, want %v", err, tt.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("ResolveLink: %v", err)
			}

			realRoot, _ := filepath.EvalSymlinks(root)
			if want := filepath.Join(realRoot, filepath.FromSlash(tt.want)); got != want {
				t.Errorf("ResolveLink = %q, want %q", got, want)
			}
		})
	}
}

func TestCopyDir_KeepsInBoundsLinks(t *testing.T) {
	_, src := newLinkTree(t)
	mustSymlink(t, ".nvmrc", filepath.Join(src, ".node-version"))
	mustSymlink(t, "real", filepath.Join(src, "alias"))

	dst := t.TempDir()

	skipped, err := osutil.CopyDir(src, dst)
	if err != nil {
		t.Fatalf("CopyDir: %v", err)
	}

	if len(skipped) != 0 {
		t.Errorf("skipped = %v, want none", skipped)
	}

	for link, want := range map[string]string{".node-version": ".nvmrc", "alias": "real"} {
		got, err := os.Readlink(filepath.Join(dst, link))
		if err != nil {
			t.Fatalf("%s is not a link: %v", link, err)
		}

		if got != want {
			t.Errorf("%s → %q, want %q", link, got, want)
		}
	}

	// A directory link is recreated, not descended into.
	if _, err := os.Lstat(filepath.Join(dst, "alias", "f.txt")); err != nil {
		t.Errorf("alias/f.txt should resolve through the link: %v", err)
	}
}

func TestCopyDir_SkipsLinksThatDoNotResolveInside(t *testing.T) {
	parent, src := newLinkTree(t)
	mustSymlink(t, filepath.Join(parent, "outside.txt"), filepath.Join(src, "abs"))
	mustSymlink(t, "../outside.txt", filepath.Join(src, "rel"))
	mustSymlink(t, "nope", filepath.Join(src, "broken"))

	dst := t.TempDir()

	skipped, err := osutil.CopyDir(src, dst)
	if err != nil {
		t.Fatalf("CopyDir: %v", err)
	}

	want := map[string]error{"abs": specs.ErrSymlinkOutside, "rel": specs.ErrSymlinkOutside, "broken": specs.ErrSymlinkDangling}
	if len(skipped) != len(want) {
		t.Fatalf("skipped = %v, want %d links", skipped, len(want))
	}

	for _, s := range skipped {
		if !errors.Is(s, want[s.Path]) {
			t.Errorf("%s: error = %v, want %v", s.Path, s.Err, want[s.Path])
		}

		if _, err := os.Lstat(filepath.Join(dst, s.Path)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s should not be in the copy: %v", s.Path, err)
		}
	}
}

func TestCopyDir_FollowsALinkedRoot(t *testing.T) {
	_, src := newLinkTree(t)

	linked := filepath.Join(t.TempDir(), "linked")
	mustSymlink(t, src, linked)

	dst := t.TempDir()

	if _, err := osutil.CopyDir(linked, dst); err != nil {
		t.Fatalf("CopyDir: %v", err)
	}

	info, err := os.Lstat(filepath.Join(dst, ".nvmrc"))
	if err != nil || !info.Mode().IsRegular() {
		t.Errorf(".nvmrc should be copied as a regular file: %v", err)
	}
}

func TestCopyDir_ReplacesAFileWithALink(t *testing.T) {
	_, src := newLinkTree(t)
	mustSymlink(t, ".nvmrc", filepath.Join(src, ".node-version"))

	dst := t.TempDir()
	mustWrite(t, filepath.Join(dst, ".node-version"), "old")

	if _, err := osutil.CopyDir(src, dst); err != nil {
		t.Fatalf("CopyDir: %v", err)
	}

	if got, err := os.Readlink(filepath.Join(dst, ".node-version")); err != nil || got != ".nvmrc" {
		t.Errorf(".node-version → %q (%v), want .nvmrc", got, err)
	}
}
