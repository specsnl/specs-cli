package template_test

import (
	"os"
	"path/filepath"
	"testing"

	pkgtemplate "github.com/specsnl/specs-cli/pkg/template"
)

func TestLoadVerbatim_MissingFile(t *testing.T) {
	dir := t.TempDir()
	rules, err := pkgtemplate.LoadVerbatim(dir)
	if err != nil {
		t.Fatalf("unexpected error for missing .specsverbatim: %v", err)
	}
	if rules.Matches("anything.txt") {
		t.Error("empty rules should not match anything")
	}
}

func writeVerbatimFile(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, ".specsverbatim"), []byte(content), 0644); err != nil {
		t.Fatalf("writeVerbatimFile: %v", err)
	}
}

func TestVerbatimMatches(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		{"exact filename", "composer.lock", "composer.lock", true},
		{"exact no match", "composer.lock", "composer.json", false},
		{"wildcard extension", "*.min.js", "dist/app.min.js", true},
		{"wildcard no match", "*.min.js", "dist/app.js", false},
		{"glob double-star", "vendor/**", "vendor/autoload.php", true},
		{"nested double-star", "vendor/**", "vendor/composer/autoload.php", true},
		{"comment ignored", "# composer.lock", "composer.lock", false},
		{"blank line ignored", "\n", "anything.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeVerbatimFile(t, dir, tt.pattern+"\n")
			rules, err := pkgtemplate.LoadVerbatim(dir)
			if err != nil {
				t.Fatalf("LoadVerbatim: %v", err)
			}
			got := rules.Matches(tt.path)
			if got != tt.want {
				t.Errorf("Matches(%q) with pattern %q = %v, want %v", tt.path, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestLoadVerbatim_ComplexVerbatimFile(t *testing.T) {
	dir := t.TempDir()
	writeVerbatimFile(t, dir, `# Lock files — always copy verbatim
composer.lock
package-lock.json

# Built assets — never template these
dist/**
build/**

# Minified files anywhere in the tree
*.min.js
*.min.css

# A specific nested path
config/secrets/credentials.json
`)

	rules, err := pkgtemplate.LoadVerbatim(dir)
	if err != nil {
		t.Fatalf("LoadVerbatim: %v", err)
	}

	shouldMatch := []string{
		"composer.lock",
		"package-lock.json",
		"dist/app.js",
		"dist/css/main.css",
		"dist/fonts/roboto.woff2",
		"build/index.html",
		"build/assets/logo.png",
		"app.min.js",
		"src/vendor/lib.min.js",
		"app.min.css",
		"config/secrets/credentials.json",
	}

	shouldNotMatch := []string{
		"composer.json",
		"package.json",
		"src/app.js",
		"src/main.css",
		"README.md",
		"config/app.php",
		"config/secrets/other.json",
	}

	for _, path := range shouldMatch {
		if !rules.Matches(path) {
			t.Errorf("expected %q to be matched, but it was not", path)
		}
	}
	for _, path := range shouldNotMatch {
		if rules.Matches(path) {
			t.Errorf("expected %q not to be matched, but it was", path)
		}
	}
}

func TestLoadVerbatim_MultiplePatterns(t *testing.T) {
	dir := t.TempDir()
	writeVerbatimFile(t, dir, "composer.lock\npackage-lock.json\n")
	rules, err := pkgtemplate.LoadVerbatim(dir)
	if err != nil {
		t.Fatalf("LoadVerbatim: %v", err)
	}
	if !rules.Matches("composer.lock") {
		t.Error("should match composer.lock")
	}
	if !rules.Matches("package-lock.json") {
		t.Error("should match package-lock.json")
	}
	if rules.Matches("composer.json") {
		t.Error("should not match composer.json")
	}
}
