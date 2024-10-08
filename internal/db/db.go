// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.27.0

package db

import (
	"context"
	"database/sql"
	"fmt"
)

type DBTX interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	PrepareContext(context.Context, string) (*sql.Stmt, error)
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}

func New(db DBTX) *Queries {
	return &Queries{db: db}
}

func Prepare(ctx context.Context, db DBTX) (*Queries, error) {
	q := Queries{db: db}
	var err error
	if q.dequeueStmt, err = db.PrepareContext(ctx, dequeue); err != nil {
		return nil, fmt.Errorf("error preparing query Dequeue: %w", err)
	}
	if q.enqueueStmt, err = db.PrepareContext(ctx, enqueue); err != nil {
		return nil, fmt.Errorf("error preparing query Enqueue: %w", err)
	}
	if q.getOriginsStmt, err = db.PrepareContext(ctx, getOrigins); err != nil {
		return nil, fmt.Errorf("error preparing query GetOrigins: %w", err)
	}
	if q.getPageStmt, err = db.PrepareContext(ctx, getPage); err != nil {
		return nil, fmt.Errorf("error preparing query GetPage: %w", err)
	}
	if q.getPagesForTermStmt, err = db.PrepareContext(ctx, getPagesForTerm); err != nil {
		return nil, fmt.Errorf("error preparing query GetPagesForTerm: %w", err)
	}
	if q.getTermStmt, err = db.PrepareContext(ctx, getTerm); err != nil {
		return nil, fmt.Errorf("error preparing query GetTerm: %w", err)
	}
	if q.insertOriginStmt, err = db.PrepareContext(ctx, insertOrigin); err != nil {
		return nil, fmt.Errorf("error preparing query InsertOrigin: %w", err)
	}
	if q.insertPageStmt, err = db.PrepareContext(ctx, insertPage); err != nil {
		return nil, fmt.Errorf("error preparing query InsertPage: %w", err)
	}
	if q.insertPageTermStmt, err = db.PrepareContext(ctx, insertPageTerm); err != nil {
		return nil, fmt.Errorf("error preparing query InsertPageTerm: %w", err)
	}
	if q.insertTermStmt, err = db.PrepareContext(ctx, insertTerm); err != nil {
		return nil, fmt.Errorf("error preparing query InsertTerm: %w", err)
	}
	if q.isIndexedStmt, err = db.PrepareContext(ctx, isIndexed); err != nil {
		return nil, fmt.Errorf("error preparing query IsIndexed: %w", err)
	}
	if q.updatePageStmt, err = db.PrepareContext(ctx, updatePage); err != nil {
		return nil, fmt.Errorf("error preparing query UpdatePage: %w", err)
	}
	return &q, nil
}

func (q *Queries) Close() error {
	var err error
	if q.dequeueStmt != nil {
		if cerr := q.dequeueStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing dequeueStmt: %w", cerr)
		}
	}
	if q.enqueueStmt != nil {
		if cerr := q.enqueueStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing enqueueStmt: %w", cerr)
		}
	}
	if q.getOriginsStmt != nil {
		if cerr := q.getOriginsStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing getOriginsStmt: %w", cerr)
		}
	}
	if q.getPageStmt != nil {
		if cerr := q.getPageStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing getPageStmt: %w", cerr)
		}
	}
	if q.getPagesForTermStmt != nil {
		if cerr := q.getPagesForTermStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing getPagesForTermStmt: %w", cerr)
		}
	}
	if q.getTermStmt != nil {
		if cerr := q.getTermStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing getTermStmt: %w", cerr)
		}
	}
	if q.insertOriginStmt != nil {
		if cerr := q.insertOriginStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing insertOriginStmt: %w", cerr)
		}
	}
	if q.insertPageStmt != nil {
		if cerr := q.insertPageStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing insertPageStmt: %w", cerr)
		}
	}
	if q.insertPageTermStmt != nil {
		if cerr := q.insertPageTermStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing insertPageTermStmt: %w", cerr)
		}
	}
	if q.insertTermStmt != nil {
		if cerr := q.insertTermStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing insertTermStmt: %w", cerr)
		}
	}
	if q.isIndexedStmt != nil {
		if cerr := q.isIndexedStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing isIndexedStmt: %w", cerr)
		}
	}
	if q.updatePageStmt != nil {
		if cerr := q.updatePageStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing updatePageStmt: %w", cerr)
		}
	}
	return err
}

func (q *Queries) exec(ctx context.Context, stmt *sql.Stmt, query string, args ...interface{}) (sql.Result, error) {
	switch {
	case stmt != nil && q.tx != nil:
		return q.tx.StmtContext(ctx, stmt).ExecContext(ctx, args...)
	case stmt != nil:
		return stmt.ExecContext(ctx, args...)
	default:
		return q.db.ExecContext(ctx, query, args...)
	}
}

func (q *Queries) query(ctx context.Context, stmt *sql.Stmt, query string, args ...interface{}) (*sql.Rows, error) {
	switch {
	case stmt != nil && q.tx != nil:
		return q.tx.StmtContext(ctx, stmt).QueryContext(ctx, args...)
	case stmt != nil:
		return stmt.QueryContext(ctx, args...)
	default:
		return q.db.QueryContext(ctx, query, args...)
	}
}

func (q *Queries) queryRow(ctx context.Context, stmt *sql.Stmt, query string, args ...interface{}) *sql.Row {
	switch {
	case stmt != nil && q.tx != nil:
		return q.tx.StmtContext(ctx, stmt).QueryRowContext(ctx, args...)
	case stmt != nil:
		return stmt.QueryRowContext(ctx, args...)
	default:
		return q.db.QueryRowContext(ctx, query, args...)
	}
}

type Queries struct {
	db                  DBTX
	tx                  *sql.Tx
	dequeueStmt         *sql.Stmt
	enqueueStmt         *sql.Stmt
	getOriginsStmt      *sql.Stmt
	getPageStmt         *sql.Stmt
	getPagesForTermStmt *sql.Stmt
	getTermStmt         *sql.Stmt
	insertOriginStmt    *sql.Stmt
	insertPageStmt      *sql.Stmt
	insertPageTermStmt  *sql.Stmt
	insertTermStmt      *sql.Stmt
	isIndexedStmt       *sql.Stmt
	updatePageStmt      *sql.Stmt
}

func (q *Queries) WithTx(tx *sql.Tx) *Queries {
	return &Queries{
		db:                  tx,
		tx:                  tx,
		dequeueStmt:         q.dequeueStmt,
		enqueueStmt:         q.enqueueStmt,
		getOriginsStmt:      q.getOriginsStmt,
		getPageStmt:         q.getPageStmt,
		getPagesForTermStmt: q.getPagesForTermStmt,
		getTermStmt:         q.getTermStmt,
		insertOriginStmt:    q.insertOriginStmt,
		insertPageStmt:      q.insertPageStmt,
		insertPageTermStmt:  q.insertPageTermStmt,
		insertTermStmt:      q.insertTermStmt,
		isIndexedStmt:       q.isIndexedStmt,
		updatePageStmt:      q.updatePageStmt,
	}
}
