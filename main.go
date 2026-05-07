package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/specsnl/specs-cli/pkg/cmd"
	"github.com/specsnl/specs-cli/pkg/util/exit"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app := cmd.NewApp()
	if err := cmd.ExecuteContext(ctx, app); err != nil {
		var exitErr *exit.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.Code)
		}
		app.Output.Error("%v", err)
		os.Exit(exit.Error)
	}
}
