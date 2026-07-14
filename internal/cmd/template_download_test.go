package cmd

import (
	"errors"
	"testing"

	"github.com/specsnl/specs-cli/internal/specs"
)

func TestDownload_LocalSourceRejected(t *testing.T) {
	withTempRegistry(t)

	_, err := executeCmd("template", "download", "./local-path", "my-tpl")
	if err == nil {
		t.Fatal("expected error when passing a local path to download")
	}
}

func TestDownload_LocalSource_IsErrLocalSource(t *testing.T) {
	withTempRegistry(t)

	_, err := executeCmd("template", "download", "./local-path", "my-tpl")
	if !errors.Is(err, specs.ErrLocalSource) {
		t.Errorf("expected ErrLocalSource, got %v", err)
	}
}

func TestDownload_AddAlias(t *testing.T) {
	withTempRegistry(t)

	// The "add" alias resolves to the download command, so a local source is
	// rejected the same way as `template download`.
	_, err := executeCmd("template", "add", "./local-path", "my-tpl")
	if !errors.Is(err, specs.ErrLocalSource) {
		t.Errorf("expected ErrLocalSource via add alias, got %v", err)
	}
}
