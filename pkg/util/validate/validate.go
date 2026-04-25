package validate

import (
	"fmt"
	"regexp"
)

var tagPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Tag returns nil if name is a valid template tag, otherwise a descriptive error.
// Valid tags contain only letters, digits, hyphens, and underscores.
func Tag(name string) error {
	if name == "" {
		return fmt.Errorf("tag must not be empty")
	}
	if !tagPattern.MatchString(name) {
		return fmt.Errorf("tag %q contains invalid characters (allowed: a-z A-Z 0-9 _ -)", name)
	}
	return nil
}
