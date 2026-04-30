package main

import (
	"os"

	"github.com/specsnl/specs-cli/pkg/cmd"
	"github.com/specsnl/specs-cli/pkg/util/exit"
	"github.com/specsnl/specs-cli/pkg/util/output"
)

func main() {
	app := cmd.NewApp()
	if err := cmd.Execute(app); err != nil {
		output.Error("%v", err)
		os.Exit(exit.Error)
	}
}
