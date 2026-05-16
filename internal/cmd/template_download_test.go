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
