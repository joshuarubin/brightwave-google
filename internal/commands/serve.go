package commands

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/joshuarubin/brightwave-google/internal/server"
)

// serve is an object used by the serve command to contain configuration and
// start the grpc server
type serve struct {
	cfg             server.Config
	shutdownTimeout time.Duration
	srv             *server.Server
}

// Serve returns the gen cobra command
func Serve() *cobra.Command {
	var s serve

	cmd := cobra.Command{
		Use:   "serve",
		Short: "Start the google server and listen for connections",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return s.serve(cmd.Context())
		},
	}

	s.flags(&cmd)

	return &cmd
}

const DefaultShutdownTimeout = 30 * time.Second

// flags sets the flags for the serve command
func (s *serve) flags(cmd *cobra.Command) {
	s.cfg.Flags(cmd)
	cmd.Flags().DurationVar(&s.shutdownTimeout, "shutdown-timeout", DefaultShutdownTimeout, "time to wait for connections to close before forcing shutdown")
}

// serve constructs the server, begins listening and catches signals in order to
// gracefully stop the server
func (s *serve) serve(ctx context.Context) error {
	var err error
	if s.srv, err = server.New(ctx, s.cfg); err != nil {
		return err
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	done := make(chan struct{})

	go func() {
		defer close(done)
		err = s.srv.Serve(ctx)
	}()

	select {
	case <-done:
		return err
	case sig := <-sigCh:
		slog.Warn("caught signal", "sig", sig)
		return s.gracefulStop()
	case <-ctx.Done():
		slog.Warn("application context done", "err", ctx.Err())
		return s.gracefulStop()
	}
}

// gracefulStop is used to stop the server, waiting for client connections to
// close, but no more than the given shutdown timeout
func (s *serve) gracefulStop() error {
	ctx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	defer cancel()

	done := make(chan struct{})

	go func() {
		defer close(done)
		s.srv.GracefulStop()
	}()

	select {
	case <-done:
		slog.Info("shutdown gracefully")
		return nil
	case <-ctx.Done():
		slog.Warn("timed out waiting to shutdown")
		return ctx.Err()
	}
}
