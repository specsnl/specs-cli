package cmd

import (
	"testing"
)

func TestDownload_LocalSourceRejected(t *testing.T) {
	withTempRegistry(t)

	_, err := executeCmd("template", "download", "./local-path", "my-tag")
	if err == nil {
		t.Fatal("expected error when passing a local path to download")
	}
}
