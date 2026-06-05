package template

import (
	"log/slog"
	texttemplate "text/template"

	"github.com/go-sprout/sprout"
	sproutchecksum "github.com/go-sprout/sprout/registry/checksum"
	sproutconversion "github.com/go-sprout/sprout/registry/conversion"
	sproutcrypto "github.com/go-sprout/sprout/registry/crypto"
	sproutencoding "github.com/go-sprout/sprout/registry/encoding"
	sproutenv "github.com/go-sprout/sprout/registry/env"
	sproutfilesystem "github.com/go-sprout/sprout/registry/filesystem"
	sproutmaps "github.com/go-sprout/sprout/registry/maps"
	sproutnetwork "github.com/go-sprout/sprout/registry/network"
	sproutnumeric "github.com/go-sprout/sprout/registry/numeric"
	sproutrandom "github.com/go-sprout/sprout/registry/random"
	sproutreflect "github.com/go-sprout/sprout/registry/reflect"
	sproutregexp "github.com/go-sprout/sprout/registry/regexp"
	sproutsemver "github.com/go-sprout/sprout/registry/semver"
	sproutslices "github.com/go-sprout/sprout/registry/slices"
	sproutstd "github.com/go-sprout/sprout/registry/std"
	sproutstrings "github.com/go-sprout/sprout/registry/strings"
	sprouttime "github.com/go-sprout/sprout/registry/time"
	sproutuniqueid "github.com/go-sprout/sprout/registry/uniqueid"
)

// FuncMap returns all template functions for the given config.
// In safe mode the env and filesystem registries are excluded.
func FuncMap(cfg Config) texttemplate.FuncMap {
	registries := []sprout.Registry{
		// Standard utility registries
		sproutstd.NewRegistry(),
		sproutconversion.NewRegistry(),
		sproutnumeric.NewRegistry(),
		sproutreflect.NewRegistry(),
		// String & encoding
		sproutstrings.NewRegistry(),
		sproutencoding.NewRegistry(),
		sproutregexp.NewRegistry(),
		// Collections
		sproutslices.NewRegistry(),
		sproutmaps.NewRegistry(),
		// Time & identity
		sprouttime.NewRegistry(),
		sproutuniqueid.NewRegistry(),
		// Crypto & checksums
		sproutcrypto.NewRegistry(),
		sproutchecksum.NewRegistry(),
		// Network
		sproutnetwork.NewRegistry(),
		// Versioning
		sproutsemver.NewRegistry(),
		// Random values
		sproutrandom.NewRegistry(),
		// Specs-specific functions
		NewSpecsRegistry(),
	}

	if !cfg.SafeMode {
		registries = append(registries,
			sproutenv.NewRegistry(),
			sproutfilesystem.NewRegistry(),
		)
	}

	handler := sprout.New(sprout.WithLogger(slog.Default()))
	mustAddRegistries(handler, registries...)
	return handler.Build()
}

// mustAddRegistries registers regs into h, panicking on programmer error (duplicate names).
func mustAddRegistries(h *sprout.DefaultHandler, regs ...sprout.Registry) {
	if err := h.AddRegistries(regs...); err != nil {
		panic("sprout registry registration failed: " + err.Error())
	}
}
