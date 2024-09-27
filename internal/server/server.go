package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"

	"github.com/spf13/cobra"

	"github.com/joshuarubin/brightwave-google/internal/crawler"
	"github.com/joshuarubin/brightwave-google/internal/db"
	"github.com/joshuarubin/brightwave-google/internal/index"
	"github.com/joshuarubin/brightwave-google/internal/queue"
	"github.com/joshuarubin/brightwave-google/internal/registrar"
	"github.com/joshuarubin/brightwave-google/internal/search"
	pb "github.com/joshuarubin/brightwave-google/pkg/proto/google/v1"
)

// Config contains the server config
type Config struct {
	Addr         string // host:port format
	TLSCertFile  string // filename
	TLSKeyFile   string // filename
	NumCrawlers  uint32
	FetchTimeout time.Duration
	DBFile       string
	ReindexDur   time.Duration
}

func (c *Config) Flags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&c.Addr, "listen-addr", ":8000", "listen address")
	cmd.Flags().StringVar(&c.TLSCertFile, "tls-cert", "", "tls server certificate file")
	cmd.Flags().StringVar(&c.TLSKeyFile, "tls-key", "", "tls server key file")
	cmd.Flags().Uint32Var(&c.NumCrawlers, "num-crawlers", 1, "number of concurrent crawlers")
	cmd.Flags().DurationVar(&c.FetchTimeout, "fetch-timeout", DefaultFetchTimeout, "timeout for fetching a page")
	cmd.Flags().StringVar(&c.DBFile, "db-file", "db.sqlite3", "sqlite3 database file")
	cmd.Flags().DurationVar(&c.ReindexDur, "reindex-duration", DefaultReindexDur, "reindex pages after this much time has elapsed")
}

type callbackKey struct {
	Table     string
	Operation int
}

// Server implements the TextGeneratorService grpc server
type Server struct {
	pb.UnimplementedGoogleServiceServer

	cfg       Config
	s         *grpc.Server
	health    *health.Server
	crawlers  []*crawler.Crawler
	index     *index.Index
	queue     *queue.Queue
	callbacks map[callbackKey][]registrar.Callback
	search    *search.Search
}

const (
	KeepaliveTime       = 30 * time.Second
	KeepaliveTimeout    = 20 * time.Second
	KeepaliveMinTime    = 15 * time.Second
	DefaultFetchTimeout = 5 * time.Second
	DefaultReindexDur   = 24 * time.Hour
)

// New constructs a new Server
func New(ctx context.Context, cfg Config) (*Server, error) {
	srv := Server{
		cfg:       cfg,
		crawlers:  make([]*crawler.Crawler, cfg.NumCrawlers),
		callbacks: map[callbackKey][]registrar.Callback{},
	}

	db, err := db.Init(ctx, cfg.DBFile, srv.onDBUpdate)
	if err != nil {
		return nil, err
	}

	srv.index = index.New(db, cfg.ReindexDur)
	srv.queue = queue.New(db, srv.index, &srv)
	srv.search = search.New(db)

	for i := range srv.crawlers {
		srv.crawlers[i] = crawler.New(i, cfg.FetchTimeout, srv.index, srv.queue)
	}

	opts := []grpc.ServerOption{
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    KeepaliveTime,
			Timeout: KeepaliveTimeout,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             KeepaliveMinTime,
			PermitWithoutStream: true,
		}),
	}

	if cfg.TLSKeyFile != "" && cfg.TLSCertFile != "" {
		tlsConfig, err := srv.tlsConfig()
		if err != nil {
			return nil, err
		}
		opts = append(opts,
			grpc.Creds(credentials.NewTLS(tlsConfig)),
		)
	}

	srv.s = grpc.NewServer(opts...)
	srv.health = health.NewServer()
	healthpb.RegisterHealthServer(srv.s, srv.health)
	reflection.Register(srv.s)

	pb.RegisterGoogleServiceServer(srv.s, &srv)

	return &srv, nil
}

func (s *Server) Register(table string, callback registrar.Callback, operation int) {
	key := callbackKey{
		Table:     table,
		Operation: operation,
	}
	s.callbacks[key] = append(s.callbacks[key], callback)
}

func (s *Server) onDBUpdate(operation int, _ string, tableName string, rowID int64) {
	key := callbackKey{
		Table:     tableName,
		Operation: operation,
	}
	if cbs, ok := s.callbacks[key]; ok {
		for _, cb := range cbs {
			cb(rowID)
		}
	}
}

// tlsConfig returns a modern, secure tls configuration that will serve the
// provide tls certificate and key
func (s *Server) tlsConfig() (*tls.Config, error) {
	crt, err := tls.LoadX509KeyPair(s.cfg.TLSCertFile, s.cfg.TLSKeyFile)
	if err != nil {
		return nil, fmt.Errorf("error loading server keypair: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{crt},
		MinVersion:   tls.VersionTLS13,
	}, nil
}

// Serve causes the server to begin listening on the configured address
func (s *Server) Serve(ctx context.Context) error {
	lis, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return err
	}

	for _, c := range s.crawlers {
		go c.Run(ctx)
	}

	slog.Info("listening", "addr", lis.Addr())

	return s.s.Serve(lis)
}

// Stop the server immediately
func (s *Server) Stop() {
	for _, c := range s.crawlers {
		c.Stop()
	}
	s.s.Stop()
}

// GracefulStop stops the server after all client connections have completed
func (s *Server) GracefulStop() {
	s.s.GracefulStop()
}

func (s *Server) Index(ctx context.Context, req *pb.IndexRequest) (*pb.IndexResponse, error) {
	u, err := url.Parse(req.GetOrigin())
	if err != nil {
		slog.Warn("error parsing url", "err", err, "url", req.GetOrigin())
		return nil, err
	}

	err = s.queue.Add(ctx, queue.Msg{
		URL:      *u,
		Origin:   *u,
		MaxDepth: req.GetK(),
	})
	if err != nil {
		return nil, err
	}

	return &pb.IndexResponse{}, nil
}

func (s *Server) Search(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	return s.search.Search(ctx, req.GetQuery())
}
