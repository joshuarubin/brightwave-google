package commands

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/joshuarubin/brightwave-google/pkg/client"
	pb "github.com/joshuarubin/brightwave-google/pkg/proto/google/v1"
)

type search struct {
	cfg client.Config
}

// Search returns the search cobra command
func Search() *cobra.Command {
	var s search

	cmd := cobra.Command{
		Use:   "search query...",
		Short: "Search for the given query",
		RunE: func(cmd *cobra.Command, args []string) error {
			return s.search(cmd.Context(), args...)
		},
	}

	s.flags(&cmd)

	return &cmd
}

// flags sets the flags for the search command
func (s *search) flags(cmd *cobra.Command) {
	s.cfg.Flags(cmd)
}

var ErrQueryRequired = errors.New("query is required")

func (s *search) search(ctx context.Context, args ...string) error {
	if len(args) == 0 {
		return ErrQueryRequired
	}

	c, err := client.New(s.cfg)
	if err != nil {
		slog.Error("error creating client", "error", err)
		return nil
	}

	resp, err := c.Search(ctx, &pb.SearchRequest{
		Query: args[0],
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error searching: %v\n", err)
		return nil
	}

	if len(resp.GetTriples()) == 0 {
		fmt.Fprintln(os.Stderr, "No results found")
		return nil
	}

	const (
		minwidth = 0
		tabwidth = 8
		padding  = 2
		padchar  = ' '
		flags    = 0
	)
	w := tabwriter.NewWriter(os.Stdout, minwidth, tabwidth, padding, padchar, flags)
	defer w.Flush()

	fmt.Fprintf(w, "URL\tDepth\tOrigins\n")
	for _, t := range resp.GetTriples() {
		fmt.Fprintf(w, "%s\t%d\t%s\n", t.GetRelevantUrl(), t.GetDepth(), strings.Join(t.GetOriginUrls(), ","))
	}

	return nil
}
