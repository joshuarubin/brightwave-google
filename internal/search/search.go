package search

import (
	"container/heap"
	"context"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/joshuarubin/brightwave-google/internal/db"
	"github.com/joshuarubin/brightwave-google/internal/text"
	pb "github.com/joshuarubin/brightwave-google/pkg/proto/google/v1"
)

type Search struct {
	db *db.DB
}

func New(d *db.DB) *Search {
	return &Search{
		db: d,
	}
}

func (s *Search) Search(ctx context.Context, query string) (*pb.SearchResponse, error) {
	q, err := text.Normalize([]byte(query))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error normalizing query: %v", err)
	}

	tokens, err := text.Tokenize(q)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error tokenizing query: %v", err)
	}

	s.db.RLock()
	defer s.db.RUnlock()

	// populate all the "RankedPage" items (those that matched at least one
	// search term)
	pages := map[int64]RankedPage{}
	for _, tok := range tokens {
		rows, err := s.db.GetPagesForTerm(ctx, tok)
		if err != nil {
			slog.Warn("error getting pages for term", "error", err, "term", tok)
			continue
		}

		for _, row := range rows {
			page := pages[row.PageID]
			if page.MatchedTerms == nil {
				page.MatchedTerms = map[string]struct{}{}
			}
			if page.Origins == nil {
				page.Origins = map[string]struct{}{}
			}
			page.PageID = row.PageID
			page.NumMatchedTerms += row.Count
			page.MatchedTerms[tok] = struct{}{}
			page.Origins[row.Origin] = struct{}{}
			pages[row.PageID] = page
		}
	}

	if len(pages) == 0 {
		return nil, status.Errorf(codes.NotFound, "no results found")
	}

	// now build a max heap out of them that ranks them according to number of
	// terms matched (i.e. relevance) and then number of unique origins (i.e.
	// importance)
	var rank PageRank
	heap.Init(&rank)

	for _, p := range pages {
		heap.Push(&rank, &p)
	}

	// for the purposes of this exercise, we'll just return, at most, the top 10
	// results

	const MaxResults = 25
	var resp pb.SearchResponse
	resp.Triples = make([]*pb.Triple, min(rank.Len(), MaxResults))

	for i := range resp.GetTriples() {
		p := heap.Pop(&rank).(*RankedPage) //nolint:forcetypeassert
		dbPage, err := s.db.GetPage(ctx, p.PageID)
		if err != nil {
			slog.Warn("error getting page", "error", err, "pageID", p.PageID)
			continue
		}
		origins, err := s.db.GetOrigins(ctx, p.PageID)
		if err != nil {
			slog.Warn("error getting origins", "error", err, "pageID", p.PageID)
			continue
		}
		resp.Triples[i] = &pb.Triple{
			RelevantUrl: dbPage.URL,
			OriginUrls:  origins,
			Depth:       uint32(dbPage.Depth),
		}
	}

	return &resp, nil
}

type RankedPage struct {
	PageID          int64
	NumMatchedTerms int64
	MatchedTerms    map[string]struct{}
	Origins         map[string]struct{}
}

type PageRank []*RankedPage

func (pq PageRank) Len() int { return len(pq) }

func (pq PageRank) Less(i, j int) bool {
	// rank the pages by the number of matching terms, then by
	// the number of unique origins
	if pq[i].NumMatchedTerms != pq[j].NumMatchedTerms {
		return pq[i].NumMatchedTerms > pq[j].NumMatchedTerms
	}

	if len(pq[i].MatchedTerms) != len(pq[j].MatchedTerms) {
		return len(pq[i].MatchedTerms) > len(pq[j].MatchedTerms)
	}

	return len(pq[i].Origins) > len(pq[j].Origins)
}

func (pq PageRank) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *PageRank) Push(x any) {
	*pq = append(*pq, x.(*RankedPage)) //nolint:forcetypeassert
}

func (pq *PageRank) Pop() any {
	n := len(*pq)
	item := (*pq)[n-1]
	*pq = (*pq)[:n-1]
	return item
}
