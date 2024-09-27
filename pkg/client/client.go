package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"os"
	"strconv"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/spf13/cobra"

	pb "github.com/joshuarubin/brightwave-google/pkg/proto/google/v1"
)

// Config contains the client config
type Config struct {
	Addr          string // host:port format
	TLSCACertFile string // filename
	Insecure      bool   // disable tls
}

func (c *Config) Flags(cmd *cobra.Command) {
	var insecure bool
	if v := os.Getenv("GOOGLE_INSECURE"); v != "" {
		if i, err := strconv.ParseBool(v); err == nil && i {
			insecure = true
		}
	}
	cmd.Flags().StringVar(&c.Addr, "addr", ":8000", "server address")
	cmd.Flags().StringVar(&c.TLSCACertFile, "tls-ca-cert", "", "tls server ca certificate file")
	cmd.Flags().BoolVar(&c.Insecure, "insecure", insecure, "enable to cause the client to connect to the server without tls ($GOOGLE_INSECURE)")
	cmd.MarkFlagsMutuallyExclusive("tls-ca-cert", "insecure")
}

// Client implements the GoogleService client
type Client struct {
	cfg      Config
	dialOpts []grpc.DialOption
	conn     *grpc.ClientConn
	client   pb.GoogleServiceClient
}

// New constructs a new Client
func New(cfg Config) (*Client, error) {
	c := Client{cfg: cfg}

	if cfg.Insecure {
		c.dialOpts = append(c.dialOpts,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
	} else {
		tc := tls.Config{
			MinVersion: tls.VersionTLS13,
		}
		if cfg.TLSCACertFile != "" {
			data, err := os.ReadFile(cfg.TLSCACertFile)
			if err != nil {
				return nil, err
			}
			tc.RootCAs = x509.NewCertPool()
			tc.RootCAs.AppendCertsFromPEM(data)
		}
		c.dialOpts = append(c.dialOpts,
			grpc.WithTransportCredentials(credentials.NewTLS(&tc)),
		)
	}

	return &c, nil
}

// dial a new grpc GoogleServiceClient. note that this is not goroutine
// safe
func (c *Client) dial() error {
	if c.conn == nil {
		c.client = nil
		conn, err := grpc.NewClient(c.cfg.Addr, c.dialOpts...)
		if err != nil {
			return err
		}
		c.conn = conn
	}
	if c.client == nil {
		c.client = pb.NewGoogleServiceClient(c.conn)
	}
	return nil
}

// Close tears down the client and all connections
func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *Client) Index(ctx context.Context, in *pb.IndexRequest) (*pb.IndexResponse, error) {
	if err := c.dial(); err != nil {
		return nil, err
	}
	return c.client.Index(ctx, in)
}

func (c *Client) Search(ctx context.Context, in *pb.SearchRequest) (*pb.SearchResponse, error) {
	if err := c.dial(); err != nil {
		return nil, err
	}
	return c.client.Search(ctx, in)
}
