package core

import (
	"context"
	"log/slog"
	"sort"
)

type Service struct {
	log   *slog.Logger
	db    DB
	words Words
}

func NewService(log *slog.Logger, db DB, words Words) (*Service, error) {
	return &Service{
		log:   log,
		db:    db,
		words: words,
	}, nil
}

func (s Service) Search(ctx context.Context, req *SearchRequest) (*SearchReply, error) {
	type scoredComic struct {
		comic DBComic
		score float64
	}

	queryWords, err := s.words.Norm(ctx, req.Phrase)
	if err != nil {
		return nil, err
	}

	allComics, err := s.db.GetAllComics(ctx)
	if err != nil {
		return nil, err
	}

	querySet := make(map[string]struct{}, len(queryWords))

	for _, word := range queryWords {
		querySet[word] = struct{}{}
	}

	scored := make([]scoredComic, 0, len(allComics))

	for _, comic := range allComics {
		score := getScore(querySet, comic.Words)

		if score > 0.0 {
			scored = append(scored, scoredComic{comic: comic, score: score})
			slog.Info("score bucket", "id", comic.ID, "score", score)
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	results := make([]Comic, 0, req.Limit)

	for _, sc := range scored {
		if len(results) >= req.Limit {
			break
		}

		results = append(results, Comic{
			ID:  sc.comic.ID,
			URL: sc.comic.URL,
		})
	}

	return &SearchReply{Comics: results}, nil
}

func getScore(querySet map[string]struct{}, comicWords []string) float64 {
	if len(comicWords) == 0 {
		return 0.0
	}

	intersection := 0

	for _, word := range comicWords {
		if _, ok := querySet[word]; ok {
			intersection++
		}
	}

	union := len(querySet) + len(comicWords) - intersection

	score := float64(intersection) / float64(union)

	return score
}
