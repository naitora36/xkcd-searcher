package core

import (
	"context"
)

//go:generate mockgen -source=ports.go -destination=mocks/isearch_core.go
type Searcher interface {
	SearchIndex(ctx context.Context, in *SearchRequest) (*SearchReply, error)
	BuildIndex(ctx context.Context) error
	DropIndex()
}
type Words interface {
	Norm(ctx context.Context, phrase string) ([]string, error)
}
type DB interface {
	GetAllComics(ctx context.Context) ([]DBComic, error)
	GetComicsByIDs(ctx context.Context, ids []int) ([]DBLightComic, error)
}
