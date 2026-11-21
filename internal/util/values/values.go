package values

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
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

// Convert turns an --arg value into the type of the variable's schema default, so that
// Flag=false is the bool false rather than a non-empty, and therefore true, string.
// A select default (a list of options) keeps the value as a string but requires it to be
// one of the options. A string default, or a key without one, keeps the value as given.
func Convert(key, value string, def any) (any, error) {
	switch d := def.(type) {
	case bool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("--arg %s=%s: %q is not a boolean (use true or false)", key, value, value)
		}

		return b, nil

	case int:
		n, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf("--arg %s=%s: %q is not an integer", key, value, value)
		}

		return n, nil

	case float64:
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, fmt.Errorf("--arg %s=%s: %q is not a number", key, value, value)
		}

		return f, nil

	case []any:
		var options []string

		for _, item := range d {
			if s, ok := item.(string); ok {
				options = append(options, s)
			}
		}

		if len(options) > 0 && !slices.Contains(options, value) {
			return nil, fmt.Errorf("--arg %s=%s: %q is not one of %s", key, value, value, strings.Join(options, ", "))
		}
	}

	return value, nil
}

// Merge applies overrides on top of base, returning a new map.
func Merge(base, overrides map[string]any) map[string]any {
	result := make(map[string]any, len(base))
	maps.Copy(result, base)
	maps.Copy(result, overrides)

	return result
}
