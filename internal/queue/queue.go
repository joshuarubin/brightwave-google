package queue

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"sync"

	"github.com/joshuarubin/brightwave-google/internal/db"
	"github.com/joshuarubin/brightwave-google/internal/index"
	"github.com/joshuarubin/brightwave-google/internal/registrar"
)

type Msg struct {
	URL      url.URL
	Origin   url.URL
	Depth    uint32
	MaxDepth uint32
}

type Queue struct {
	condMu sync.Mutex
	cond   *sync.Cond
	index  *index.Index
	db     *db.DB
}

func New(d *db.DB, i *index.Index, r registrar.Registrar) *Queue {
	q := Queue{
		index: i,
		db:    d,
	}
	q.cond = sync.NewCond(&q.condMu)

	r.Register("queue", q.onDBInsert, db.SQLITE_INSERT)

	return &q
}

func prepareMsg(item db.Queue) Msg {
	u, err := url.Parse(item.URL)
	if err != nil {
		slog.Error("error parsing url", "error", err, "url", item.URL)
		return Msg{}
	}
	u = index.CleanURL(u)

	o, err := url.Parse(item.Origin)
	if err != nil {
		slog.Error("error parsing origin url", "error", err, "url", item.URL)
		return Msg{}
	}
	o = index.CleanURL(o)

	return Msg{
		URL:      *u,
		Origin:   *o,
		Depth:    uint32(item.Depth),
		MaxDepth: uint32(item.MaxDepth),
	}
}

func (q *Queue) Next(ctx context.Context) <-chan Msg {
	ch := make(chan Msg)
	var emptiedDB bool

	go func() {
		if !emptiedDB {
			// first process queue already in db
			q.db.Lock()
			item, err := q.db.Dequeue(ctx)
			q.db.Unlock()
			switch {
			case errors.Is(err, sql.ErrNoRows):
				emptiedDB = true
			case err != nil:
				slog.Error("error dequeuing", "error", err)
				ch <- Msg{}
				return
			default:
				ch <- prepareMsg(item)
				return
			}
		}

		// there wasn't anything queued, so wait until there is

		q.condMu.Lock()
		q.cond.Wait()
		q.condMu.Unlock()

		q.db.Lock()
		item, err := q.db.Dequeue(ctx)
		q.db.Unlock()

		if err != nil {
			slog.Error("error dequeuing", "error", err)
			ch <- Msg{}
			return
		}

		ch <- prepareMsg(item)
	}()

	return ch
}

func (q *Queue) onDBInsert(_ int64) {
	q.condMu.Lock()
	q.cond.Signal()
	q.condMu.Unlock()
}

func (q *Queue) Add(ctx context.Context, msg Msg) error {
	msg.URL = *index.CleanURL(&msg.URL)
	msg.Origin = *index.CleanURL(&msg.Origin)

	q.db.RLock()

	tx, err := q.db.SQL.Begin()
	if err != nil {
		q.db.RUnlock()
		return fmt.Errorf("error starting transaction for queue add: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	queries := q.db.WithTx(tx)

	if !q.index.ShouldIndex(ctx, msg.URL, tx) {
		q.db.RUnlock()
		slog.Info("queue: not re-indexing", "url", msg.URL.String())
		return nil
	}

	q.db.RUnlock()
	q.db.Lock()
	defer q.db.Unlock()

	err = queries.Enqueue(ctx, db.EnqueueParams{
		URL:      msg.URL.String(),
		Origin:   msg.Origin.String(),
		Depth:    int64(msg.Depth),
		MaxDepth: int64(msg.MaxDepth),
	})
	if err != nil {
		return fmt.Errorf("error enqueuing to db: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("error committing enqueue transaction: %w", err)
	}

	return nil
}
