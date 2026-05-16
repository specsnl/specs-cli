package validate

import (
	"fmt"
	"regexp"
)

var namePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Name returns nil if name is a valid template name, otherwise a descriptive error.
// Valid names contain only letters, digits, hyphens, and underscores.
func Name(name string) error {
	if name == "" {
		return fmt.Errorf("name must not be empty")
	}
	if !namePattern.MatchString(name) {
		return fmt.Errorf("name %q contains invalid characters (allowed: a-z A-Z 0-9 _ -)", name)
	}
	return nil
}
