package core

import (
	"context"
	"log/slog"
	"sort"
	"sync/atomic"
)

type Service struct {
	log   *slog.Logger
	db    DB
	words Words
	index atomic.Pointer[SearchIndex]
}

func NewService(log *slog.Logger, db DB, words Words) (*Service, error) {
	service := &Service{
		log:   log,
		db:    db,
		words: words,
	}

	emptyIndex := &SearchIndex{make(map[string][]int)}
	service.index.Store(emptyIndex)

	return service, nil
}

func (s *Service) BuildIndex(ctx context.Context) error {
	comics, err := s.db.GetAllComics(ctx)
	if err != nil {
		return err
	}

	si := SearchIndex{Index: make(map[string][]int, len(comics))}
	for _, comic := range comics {
		for _, word := range comic.Words {
			si.Index[word] = append(si.Index[word], comic.ID)
		}
	}

	s.index.Store(&si)

	return nil
}

func (s *Service) SearchIndex(ctx context.Context, req *SearchRequest) (*SearchReply, error) {
	queryWords, err := s.words.Norm(ctx, req.Phrase)
	if err != nil {
		return nil, err
	}

	currentIndex := s.index.Load()

	idsSet := make([]int, 0, len(queryWords))
	for _, word := range queryWords {
		idsSet = append(idsSet, currentIndex.Index[word]...)
	}

	comicsIDs := sortByFrequency(idsSet, req.Limit)

	dbRes, err := s.db.GetComicsByIDs(ctx, comicsIDs)
	if err != nil {
		return nil, err
	}

	comics := make([]Comic, len(dbRes))
	for k, v := range dbRes {
		comics[k] = Comic(v)
	}

	res := &SearchReply{Comics: comics}

	return res, nil
}

func sortByFrequency(idsSet []int, limit int) []int {
	type scoredResult struct {
		comicID int
		score   int
	}

	counts := make(map[int]scoredResult)

	for _, id := range idsSet {
		if entry, ok := counts[id]; ok {
			entry.score++
			counts[id] = entry
		} else {
			counts[id] = scoredResult{
				comicID: id,
				score:   1,
			}
		}
	}

	scoredSlice := make([]scoredResult, 0, len(counts))
	for _, v := range counts {
		scoredSlice = append(scoredSlice, v)
	}

	sort.Slice(scoredSlice, func(i, j int) bool {
		if scoredSlice[i].score == scoredSlice[j].score {
			return scoredSlice[i].comicID > scoredSlice[j].comicID
		}
		return scoredSlice[i].score > scoredSlice[j].score
	})

	res := make([]int, 0, limit)
	for i := 0; i < len(scoredSlice) && i < limit; i++ {
		res = append(res, scoredSlice[i].comicID)
	}

	return res
}
