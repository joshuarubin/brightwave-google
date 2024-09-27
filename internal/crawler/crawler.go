package crawler

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"golang.org/x/net/html"

	"github.com/joshuarubin/brightwave-google/internal/index"
	"github.com/joshuarubin/brightwave-google/internal/queue"
)

type Crawler struct {
	id           int
	client       *http.Client
	stop         chan struct{}
	fetchTimeout time.Duration
	index        *index.Index
	queue        *queue.Queue
	logger       *slog.Logger
}

type transport struct {
	parent http.RoundTripper
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; BrightwaveBot/1.0; +http://brightwave.io)")
	return t.parent.RoundTrip(req)
}

func New(id int, fetchTimeout time.Duration, index *index.Index, queue *queue.Queue) *Crawler {
	return &Crawler{
		id:           id,
		fetchTimeout: fetchTimeout,
		index:        index,
		queue:        queue,
		stop:         make(chan struct{}),
		logger:       slog.With("crawler", id),
		client: &http.Client{
			Transport: &transport{
				parent: cleanhttp.DefaultPooledTransport(),
			},
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				// disable auto redirects
				return http.ErrUseLastResponse
			},
		},
	}
}

func (c *Crawler) Run(ctx context.Context) {
	c.logger.Info("running")
	for {
		select {
		case <-c.stop:
			return
		case msg := <-c.queue.Next(ctx):
			c.handleMsg(ctx, msg)
		}
	}
}

func (c *Crawler) handleMsg(ctx context.Context, msg queue.Msg) {
	if !c.index.ShouldIndex(ctx, msg.URL, nil) {
		c.logger.Info("crawler: not re-indexing", "url", msg.URL.String())
		return
	}

	c.logger.Info("received", "url", msg.URL.String(), "origin", msg.Origin.String(), "depth", msg.Depth, "max_depth", msg.MaxDepth)
	ctx, cancel := context.WithTimeout(ctx, c.fetchTimeout)
	defer cancel()

	resp, err := c.fetch(ctx, msg)
	switch {
	case errors.Is(err, ErrRedirectLoop):
	case errors.Is(err, ErrMaxDepth):
	case errors.Is(err, ErrRedirect):
	case err != nil:
		c.logger.Warn("error fetching", "err", err, "url", msg.URL.String())
	}
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if err = c.process(ctx, msg, resp.Body); err != nil {
		c.logger.Warn("error processing", "err", err, "url", msg.URL.String())
	}
}

func (c *Crawler) process(ctx context.Context, msg queue.Msg, body io.Reader) error {
	z := html.NewTokenizer(body)
	tags := []string{}
	var buf bytes.Buffer
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			if errors.Is(z.Err(), io.EOF) {
				c.logger.Info("processed", "url", msg.URL.String())
				return c.index.Add(ctx, index.Page{
					URL:    msg.URL,
					Origin: msg.Origin,
					Depth:  msg.Depth,
				}, buf.Bytes())
			}
			return z.Err()
		case html.TextToken:
			if len(tags) == 0 {
				continue
			}
			switch tags[len(tags)-1] {
			case "script", "style":
				// don't consider these as text data
			default:
				buf.Write(z.Text())
				buf.WriteString(" ")
			}
		case html.StartTagToken:
			t := z.Token()
			tags = append(tags, t.Data)
			if msg.Depth >= msg.MaxDepth {
				// don't add links if they will be exceed max depth
				continue
			}
			if t.Data == "a" {
				for _, a := range t.Attr {
					var link *url.URL
					var err error
					if a.Key == "href" {
						switch {
						case strings.HasPrefix(a.Val, "#"):
							// ignore fragment only urls
							continue
						case strings.HasPrefix(a.Val, "/"): // also handles // prefix
							link, err = msg.URL.Parse(a.Val)
						default:
							link, err = url.Parse(a.Val)
						}
						if err != nil {
							c.logger.Warn("error parsing link", "err", err, "url", a.Val)
							continue
						}

						c.enQueue(ctx, queue.Msg{
							URL:      *link,
							Origin:   msg.Origin,
							Depth:    msg.Depth + 1,
							MaxDepth: msg.MaxDepth,
						})
					}
				}
			}
		case html.EndTagToken:
			if len(tags) > 0 {
				tags = tags[:len(tags)-1]
			} else {
				tags = nil
			}
		}
	}
}

func (c *Crawler) Stop() {
	close(c.stop)
}

var (
	ErrRedirectLoop = errors.New("redirect loop detected")
	ErrMaxDepth     = errors.New("max depth reached")
	ErrRedirect     = errors.New("redirect found")
)

func (c *Crawler) enQueue(ctx context.Context, msg queue.Msg) {
	// do this in a goroutine to prevent deadlocks
	if err := c.queue.Add(ctx, msg); err != nil {
		c.logger.Warn("error enqueuing", "err", err, "url", msg.URL.String())
	}
}

func (c *Crawler) fetch(ctx context.Context, msg queue.Msg) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, msg.URL.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	if v := resp.Header.Get("Location"); v != "" {
		switch {
		case v == msg.URL.String():
			c.logger.Warn("redirect loop detected", "url", msg.URL.String(), "location", v)
			resp.Body.Close()
			// TODO(jrubin) index the redirect?
			return nil, ErrRedirectLoop
		case msg.Depth >= msg.MaxDepth:
			c.logger.Warn("redirect found, max depth reached", "url", msg.URL.String(), "location", v)
			resp.Body.Close()
			// TODO(jrubin) index the redirect?
			return nil, ErrMaxDepth
		default:
			u, err := url.Parse(v)
			if err != nil {
				c.logger.Warn("error parsing url", "err", err, "url", v)
				return nil, err
			}
			c.enQueue(ctx, queue.Msg{
				URL:      *u,
				Origin:   msg.Origin,
				Depth:    msg.Depth + 1,
				MaxDepth: msg.MaxDepth,
			})
			resp.Body.Close()
			return nil, ErrRedirect
		}
	}

	c.logger.Info("fetched", "url", msg.URL.String(), "status", resp.StatusCode)
	return resp, nil
}
