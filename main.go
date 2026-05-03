package main

import (
	"os"

	"github.com/specsnl/specs-cli/pkg/cmd"
	"github.com/specsnl/specs-cli/pkg/util/exit"
)

func main() {
	app := cmd.NewApp()
	if err := cmd.Execute(app); err != nil {
		app.Output.Error("%v", err)
		os.Exit(exit.Error)
	}
}
