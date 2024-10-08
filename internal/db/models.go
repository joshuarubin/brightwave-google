// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.27.0

package db

import (
	"time"
)

type Origin struct {
	ID        int64
	CreatedAt time.Time
	PageID    int64
	Origin    string
}

type Page struct {
	ID         int64
	CreatedAt  time.Time
	ModifiedAt time.Time
	URL        string
	Depth      int64
}

type PageTerm struct {
	ID        int64
	CreatedAt time.Time
	PageID    int64
	TermID    int64
	Count     int64
}

type Queue struct {
	ID        int64
	CreatedAt time.Time
	URL       string
	Origin    string
	Depth     int64
	MaxDepth  int64
}

type Term struct {
	ID        int64
	CreatedAt time.Time
	Term      string
}
