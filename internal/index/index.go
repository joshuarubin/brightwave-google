package index

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/joshuarubin/brightwave-google/internal/db"
	"github.com/joshuarubin/brightwave-google/internal/text"
)

func CleanURL(u *url.URL) *url.URL {
	u.Fragment = ""
	u.User = nil
	query := u.Query()
	for k := range query {
		if strings.HasPrefix(k, "utm_") {
			query.Del(k)
		}
	}
	u.RawQuery = query.Encode()
	return u
}

type Index struct {
	db         *db.DB
	reindexDur time.Duration
}

func New(db *db.DB, reindexDur time.Duration) *Index {
	return &Index{
		db:         db,
		reindexDur: reindexDur,
	}
}

func (i *Index) ShouldIndex(ctx context.Context, u url.URL, tx *sql.Tx) bool {
	queries := i.db.Queries
	if tx != nil {
		queries = i.db.WithTx(tx)
	} else {
		i.db.RLock()
		defer i.db.RUnlock()
	}

	expiration := time.Now().UTC().Add(-i.reindexDur)

	_, err := queries.IsIndexed(ctx, db.IsIndexedParams{
		URL:        u.String(),
		ModifiedAt: expiration,
	})
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return true
	case err != nil:
		slog.Error("error checking if should index", "error", err, "url", u.String())
		return true
	default:
		// TODO(jrubin) delete the page from the index
		return false
	}
}

type Page struct {
	URL    url.URL
	Origin url.URL
	Depth  uint32
}

func (i *Index) Add(ctx context.Context, page Page, data []byte) error {
	// normally, this should probably go into a processing queue/pipeline
	// but for the purpose of this exercise, these operations are fast enough
	// to do here
	data, err := text.Normalize(data)
	if err != nil {
		return err
	}

	// TODO(jrubin) tokenize parts of url too

	tokens, err := text.Tokenize(data)
	if err != nil {
		return err
	}

	i.db.RLock()

	tx, err := i.db.SQL.Begin()
	if err != nil {
		i.db.RUnlock()
		return fmt.Errorf("error starting transaction for index add: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// would normally want to do this before normalize too, but for this
	// exercise it just slows things down
	if !i.ShouldIndex(ctx, page.URL, tx) {
		i.db.RUnlock()
		slog.Info("index.add: not re-indexing", "url", page.URL.String())
		return nil
	}

	queries := i.db.WithTx(tx)

	i.db.RUnlock()
	i.db.Lock()
	defer i.db.Unlock()

	dbPage, err := queries.InsertPage(ctx, db.InsertPageParams{
		URL:   page.URL.String(),
		Depth: int64(page.Depth),
	})
	switch {
	case errors.Is(err, sql.ErrNoRows):
		dbPage, err = queries.UpdatePage(ctx, db.UpdatePageParams{
			URL:   page.URL.String(),
			Depth: int64(page.Depth),
		})
		if err != nil {
			return fmt.Errorf("error updating page: %w", err)
		}
	case err != nil:
		return fmt.Errorf("error inserting page: %w", err)
	}

	err = queries.InsertOrigin(ctx, db.InsertOriginParams{
		PageID: dbPage.ID,
		Origin: page.Origin.String(),
	})
	if err != nil {
		return fmt.Errorf("error inserting origin: %w", err)
	}

	// sqlite supports multi-row inserts, but sqlc doesn't seem to support that
	// yet, so we'll use inefficient one-row inserts
LOOP:
	for _, t := range tokens {
		var term db.Term
		term, err = queries.InsertTerm(ctx, t)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			term, err = queries.GetTerm(ctx, t)
			if err != nil {
				slog.Warn("error getting existing term", "error", err, "term", t)
				continue LOOP
			}
		case err != nil:
			slog.Warn("error inserting term", "error", err, "term", t)
			continue LOOP
		}

		err = queries.InsertPageTerm(ctx, db.InsertPageTermParams{
			PageID: dbPage.ID,
			TermID: term.ID,
		})
		if err != nil {
			slog.Warn("error inserting page term", "error", err, "pageID", dbPage.ID, "termID", term.ID, "term", t, "page", page.URL.String())
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("error committing index add transaction: %w", err)
	}

	slog.Info("indexed", "url", page.URL.String())

	return nil
}
