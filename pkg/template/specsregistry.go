package template

import (
	"os"
	"os/user"
	"strconv"

	"github.com/docker/go-units"
	"github.com/go-sprout/sprout"
	"github.com/sethvargo/go-password/password"
)

// SpecsRegistry is a sprout Registry that provides specs-specific template functions.
type SpecsRegistry struct {
	handler sprout.Handler
}

// NewSpecsRegistry creates a new instance of the specs registry.
func NewSpecsRegistry() *SpecsRegistry {
	return &SpecsRegistry{}
}

// UID returns the unique identifier of the registry.
func (r *SpecsRegistry) UID() string {
	return "specsnl/specs-cli.specs"
}

// LinkHandler links the sprout handler to the registry at runtime.
func (r *SpecsRegistry) LinkHandler(fh sprout.Handler) error {
	r.handler = fh
	return nil
}

// RegisterFunctions registers all specs template functions into the function map.
func (r *SpecsRegistry) RegisterFunctions(funcsMap sprout.FunctionMap) error {
	sprout.AddFunction(funcsMap, "hostname", r.Hostname)
	sprout.AddFunction(funcsMap, "username", r.Username)
	sprout.AddFunction(funcsMap, "toBinary", r.ToBinary)
	sprout.AddFunction(funcsMap, "formatFilesize", r.FormatFilesize)
	sprout.AddFunction(funcsMap, "password", r.Password)
	return nil
}

// Hostname returns the system hostname.
func (r *SpecsRegistry) Hostname() string {
	h, _ := os.Hostname()
	return h
}

// Username returns the current OS username.
func (r *SpecsRegistry) Username() string {
	u, _ := user.Current()
	if u != nil {
		return u.Username
	}
	return ""
}

// ToBinary formats an integer as a binary string.
func (r *SpecsRegistry) ToBinary(n int) string {
	return strconv.FormatInt(int64(n), 2)
}

// FormatFilesize formats a byte count as a human-readable size string (e.g. "1.2 MB").
func (r *SpecsRegistry) FormatFilesize(bytes float64) string {
	return units.HumanSize(bytes)
}

// Password generates a secure random password.
func (r *SpecsRegistry) Password(length, digits, symbols int, noUpper, allowRepeat bool) string {
	p, _ := password.Generate(length, digits, symbols, noUpper, allowRepeat)
	return p
}
