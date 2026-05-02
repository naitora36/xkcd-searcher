package core

import "context"

//go:generate mockgen -source=ports.go -destination=mocks/search_core.go
type Searcher interface {
	Search(context.Context, *SearchRequest) (*SearchReply, error)
}

type Words interface {
	Norm(ctx context.Context, phrase string) ([]string, error)
}
type DB interface {
	GetAllComics(ctx context.Context) ([]DBComic, error)
}
