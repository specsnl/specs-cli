package template

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/danwakefield/fnmatch"

	"github.com/specsnl/specs-cli/pkg/specs"
)

// VerbatimRules holds glob patterns loaded from .specsverbatim.
type VerbatimRules struct {
	patterns []string
}

// LoadVerbatim reads .specsverbatim from templateRoot. Returns an empty VerbatimRules
// (no patterns, no error) if the file does not exist.
func LoadVerbatim(templateRoot string) (*VerbatimRules, error) {
	path := filepath.Join(templateRoot, specs.VerbatimFile)
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return &VerbatimRules{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var patterns []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return &VerbatimRules{patterns: patterns}, scanner.Err()
}

// Matches reports whether path (relative to the template/ root, using forward
// slashes) matches any verbatim pattern.
func (r *VerbatimRules) Matches(path string) bool {
	for _, pattern := range r.patterns {
		if fnmatch.Match(pattern, path, 0) {
			return true
		}
	}
	return false
}
