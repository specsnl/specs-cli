package main

import (
	"os"

	"github.com/specsnl/specs-cli/pkg/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
