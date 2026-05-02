package core

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
)

type Service struct {
	log         *slog.Logger
	db          DB
	xkcd        XKCD
	words       Words
	concurrency int
	updating    atomic.Bool
}

func NewService(log *slog.Logger, db DB, xkcd XKCD, words Words, concurrency int) (*Service, error) {
	if concurrency < 1 {
		return nil, fmt.Errorf("wrong concurrency specified: %d", concurrency)
	}
	return &Service{
		log:         log,
		db:          db,
		xkcd:        xkcd,
		words:       words,
		concurrency: concurrency,
	}, nil
}

func (s *Service) Update(ctx context.Context) (err error) {
	if !s.updating.CompareAndSwap(false, true) {
		return ErrAlreadyRunning
	}
	defer s.updating.Store(false)

	existIdsSlice, err := s.db.IDs(ctx)
	if err != nil {
		return fmt.Errorf("db get ids error: %w", err)
	}

	existIds := make(map[int]struct{}, len(existIdsSlice))
	for _, v := range existIdsSlice {
		existIds[v] = struct{}{}
	}

	numOfComics, err := s.xkcd.LastID(ctx)
	if err != nil {
		return fmt.Errorf("xkcd get last id error: %w", err)
	}

	in := make(chan int)
	g, gCtx := errgroup.WithContext(ctx)

	go func() {
		defer close(in)
		for i := 1; i <= numOfComics; i++ {
			select {
			case <-gCtx.Done():
				return
			case in <- i:
			}
		}
	}()

	for w := 1; w <= s.concurrency; w++ {
		g.Go(func() error {
			for x := range in {
				if _, ok := existIds[x]; ok {
					continue
				}

				comicsInfo, err := s.fetchWithRetry(gCtx, x)
				if err != nil {
					s.log.Warn("skipping comic due to fetch error", "id", x, "error", err)
					continue
				}

				info := comicsInfo.Title + " " + comicsInfo.Description
				// Довольно таки грубо обрезал, лучше бы конечно это как-то более аккуратно по словам обрезать, но пока так
				if len(info) > 2550 {
					info = info[:2550]
				}
				words, err := s.words.Norm(gCtx, info)
				if err != nil {
					s.log.Warn("skipping comic due to normalize error", "id", x, "error", err)
					continue
				}

				comics := Comics{
					ID:    comicsInfo.ID,
					URL:   comicsInfo.URL,
					Words: words,
				}

				err = s.db.Add(gCtx, comics)
				if err != nil {
					return fmt.Errorf("db add comics error: %w", err)
				}
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}
	return nil
}

func (s *Service) Stats(ctx context.Context) (ServiceStats, error) {
	dbStats, err := s.db.Stats(ctx)
	if err != nil {
		return ServiceStats{}, fmt.Errorf("failed to get db stats: %w", err)
	}

	lastID, err := s.xkcd.LastID(ctx)
	if err != nil {
		s.log.Warn("could not get last id from xkcd, using 0", "error", err)
		lastID = 0
	}

	return ServiceStats{
		DBStats:     dbStats,
		ComicsTotal: lastID,
	}, nil
}

func (s *Service) Status(ctx context.Context) ServiceStatus {
	if s.updating.Load() {
		return StatusRunning
	}
	return StatusIdle
}

func (s *Service) Drop(ctx context.Context) error {
	if !s.updating.CompareAndSwap(false, true) {
		return ErrAlreadyRunning
	}
	defer s.updating.Store(false)

	return s.db.Drop(ctx)
}

func (s *Service) fetchWithRetry(ctx context.Context, id int) (XKCDInfo, error) {
	baseDelay := 500 * time.Millisecond
	maxAttempts := 3

	var lastErr error
	for attempt := range maxAttempts {
		info, err := s.xkcd.Get(ctx, id)
		if err == nil {
			return info, nil
		}

		lastErr = err
		s.log.Warn("fetch failed, retrying", "id", id, "attempt", attempt+1, "error", err)

		if attempt == maxAttempts-1 {
			break
		}

		delay := baseDelay * (1 << attempt)
		jitter := rand.N(delay / 2)

		select {
		case <-time.After(delay + jitter):
		case <-ctx.Done():
			return XKCDInfo{}, ctx.Err()
		}
	}

	return XKCDInfo{}, fmt.Errorf("all attempts failed: %w", lastErr)
}
