package values

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadFile reads a JSON or YAML file and returns a flat map of key/value overrides.
// Files with a .yaml or .yml extension are parsed as YAML; all others as JSON.
func LoadFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading values file %q: %w", path, err)
	}
	var m map[string]any
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".yaml" || ext == ".yml" {
		if err := yaml.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("parsing values file %q: %w", path, err)
		}
	} else {
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("parsing values file %q: %w", path, err)
		}
	}
	return m, nil
}

// ParseArg parses a "Key=Value" string into its key and value parts.
func ParseArg(arg string) (key, value string, err error) {
	parts := strings.SplitN(arg, "=", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("--arg %q must be in Key=Value form", arg)
	}
	return parts[0], parts[1], nil
}

// Merge applies overrides on top of base, returning a new map.
func Merge(base, overrides map[string]any) map[string]any {
	result := make(map[string]any, len(base))
	for k, v := range base {
		result[k] = v
	}
	for k, v := range overrides {
		result[k] = v
	}
	return result
}
