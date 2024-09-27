package main

import (
	"context"
	"os"

	"github.com/spf13/cobra"

	"github.com/joshuarubin/brightwave-google/internal/commands"
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	root := cobra.Command{
		Use:   "google",
		Short: "Simple Google API server",
	}

	root.AddCommand(commands.Index())
	root.AddCommand(commands.Search())
	root.AddCommand(commands.Serve())

	ctx := context.Background()
	return root.ExecuteContext(ctx)
}
