package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
)

const maxLenDescription = 3000

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

// Попробовал чуть подразбить логику на более мелкие функции, не знаю насколько удачно получилось...
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
			if _, ok := existIds[i]; ok {
				continue
			}
			select {
			case <-gCtx.Done():
				return
			case in <- i:
			}
		}
	}()

	for w := 1; w <= s.concurrency; w++ {
		g.Go(func() error {
			for id := range in {
				err := s.processOne(gCtx, id)
				if err != nil {
					if errors.Is(err, ErrSkipped) {
						continue
					}
					return err
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

func (s *Service) processOne(ctx context.Context, id int) error {
	comicsInfo, err := s.fetchWithRetry(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			err = s.addEmptyComics(ctx, id)
			if err != nil {
				return err
			}
			return ErrSkipped
		}
		s.log.Warn("skipping comic due to fetch error", "id", id, "error", err)
		return ErrSkipped
	}

	info := limitWords(comicsInfo.Description, maxLenDescription)

	words, err := s.words.Norm(ctx, info)
	if err != nil {
		s.log.Warn("skipping comic due to normalize error", "id", id, "error", err)
		return ErrSkipped
	}

	comics := Comics{
		ID:    comicsInfo.ID,
		URL:   comicsInfo.URL,
		Words: words,
	}

	err = s.db.Add(ctx, comics)
	if err != nil {
		return fmt.Errorf("db add comics error: %w", err)
	}
	return nil
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

func (s *Service) addEmptyComics(ctx context.Context, id int) error {
	emptyComics := Comics{
		ID:    id,
		URL:   "http://empty_comics",
		Words: []string{},
	}

	err := s.db.Add(ctx, emptyComics)
	if err != nil {
		return fmt.Errorf("db add comics error: %w", err)
	}
	return nil
}

func limitWords(description string, maxLen int) string {
	runes := []rune(description)

	if len(runes) <= maxLen {
		return description
	}

	truncated := runes[:maxLen]

	lastSpace := -1
	for i := len(truncated) - 1; i >= 0; i-- {
		if truncated[i] == ' ' {
			lastSpace = i
			break
		}
	}

	if lastSpace > 0 {
		return string(truncated[:lastSpace])
	}

	return string(truncated)
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
