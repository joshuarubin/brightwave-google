package commands

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/joshuarubin/brightwave-google/pkg/client"
	pb "github.com/joshuarubin/brightwave-google/pkg/proto/google/v1"
)

type index struct {
	cfg client.Config
}

// Index returns the index cobra command
func Index() *cobra.Command {
	var i index

	cmd := cobra.Command{
		Use:   "index url max-depth",
		Short: "Index the given url",
		RunE: func(cmd *cobra.Command, args []string) error {
			return i.index(cmd.Context(), args...)
		},
	}

	i.flags(&cmd)

	return &cmd
}

// flags sets the flags for the index command
func (i *index) flags(cmd *cobra.Command) {
	i.cfg.Flags(cmd)
}

var (
	ErrURLRequired      = errors.New("url is required")
	ErrMaxDepthRequired = errors.New("max depth is required")
)

func (i *index) index(ctx context.Context, args ...string) error {
	if len(args) < 1 {
		return ErrURLRequired
	}
	if len(args) < 2 { //nolint:mnd
		return ErrMaxDepthRequired
	}

	u, err := url.Parse(args[0])
	if err != nil {
		return fmt.Errorf("error parsing url: %w", err)
	}

	maxDepth, err := strconv.ParseUint(args[1], 10, 32)
	if err != nil {
		return fmt.Errorf("error parsing max-depth: %w", err)
	}

	c, err := client.New(i.cfg)
	if err != nil {
		return fmt.Errorf("error creating client: %w", err)
	}

	_, err = c.Index(ctx, &pb.IndexRequest{
		Origin: u.String(),
		K:      uint32(maxDepth),
	})
	if err != nil {
		return fmt.Errorf("error indexing url: %w", err)
	}

	return nil
}
