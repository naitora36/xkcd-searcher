package core

import "context"

var LimitWords = limitWords

func (s *Service) FetchWithRetry(ctx context.Context, id int) (XKCDInfo, error) {
	return s.fetchWithRetry(ctx, id)
}

func (s *Service) ProcessOne(ctx context.Context, id int) error {
	return s.processOne(ctx, id)
}

func (s *Service) AddEmptyComics(ctx context.Context, id int) error {
	return s.addEmptyComics(ctx, id)
}

func (s *Service) SetUpdating(val bool) {
	s.updating.Store(val)
}
