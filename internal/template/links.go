package template

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/specsnl/specs-cli/internal/specs"
	"github.com/specsnl/specs-cli/internal/util/osutil"
)

// CheckLinks returns an error naming every symlink under templateRoot/template/
// that resolves outside it. A cloned template must pass it before it is used or
// registered: unlike a local one, its links were written by someone else.
// Dangling links pass; Execute skips them with a warning.
func CheckLinks(templateRoot string) error {
	srcRoot := filepath.Join(templateRoot, specs.TemplateDirFile)
	if _, err := os.Lstat(srcRoot); errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	var errs []error

	err := filepath.WalkDir(srcRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !osutil.IsLink(d) {
			return nil
		}

		_, err = osutil.LinkTarget(srcRoot, p)

		var linkErr *osutil.LinkError
		if errors.As(err, &linkErr) {
			if errors.Is(err, specs.ErrSymlinkOutside) {
				linkErr.Path = path.Join(specs.TemplateDirFile, linkErr.Path)
				errs = append(errs, linkErr)
			}

			return nil
		}

		return err
	})
	if err != nil {
		return err
	}

	return errors.Join(errs...)
}
