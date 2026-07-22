package osutil

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// CopyDir recursively copies the directory tree at src into dst.
// dst is created if it does not exist. Existing files in dst are overwritten.
func CopyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}

		return copyFile(path, target)
	})
}

func copyFile(src, dst string) (err error) {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}

	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}

	// A failed Close on a writable file can mean buffered data was not flushed,
	// so surface it when no earlier error already took precedence.
	defer func() {
		if cerr := out.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}

	return os.Chmod(dst, info.Mode())
}
