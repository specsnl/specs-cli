package osutil_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/specsnl/specs-cli/pkg/util/osutil"
)

func TestCopyDir_PreservesStructure(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Create nested structure in src
	if err := os.MkdirAll(filepath.Join(src, "a", "b"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "a", "b", "file.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "root.txt"), []byte("root"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := osutil.CopyDir(src, dst); err != nil {
		t.Fatalf("CopyDir: %v", err)
	}

	for _, path := range []string{"a/b/file.txt", "root.txt"} {
		if _, err := os.Stat(filepath.Join(dst, path)); err != nil {
			t.Errorf("expected %q to exist in dst: %v", path, err)
		}
	}
}

func TestCopyDir_PreservesContent(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	content := []byte("expected content")
	if err := os.WriteFile(filepath.Join(src, "file.txt"), content, 0644); err != nil {
		t.Fatal(err)
	}

	if err := osutil.CopyDir(src, dst); err != nil {
		t.Fatalf("CopyDir: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dst, "file.txt"))
	if err != nil {
		t.Fatalf("reading dst file: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("content = %q, want %q", got, content)
	}
}

func TestCopyDir_OverwritesExisting(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	if err := os.WriteFile(filepath.Join(src, "file.txt"), []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "file.txt"), []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := osutil.CopyDir(src, dst); err != nil {
		t.Fatalf("CopyDir: %v", err)
	}

	got, _ := os.ReadFile(filepath.Join(dst, "file.txt"))
	if string(got) != "new" {
		t.Errorf("content = %q, want %q", got, "new")
	}
}
