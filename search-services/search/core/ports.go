package core

import "context"

type Searcher interface {
	Search(context.Context, *SearchRequest) (*SearchReply, error)
}
type Words interface {
	Norm(ctx context.Context, phrase string) ([]string, error)
}
type DB interface {
	GetAllComics(ctx context.Context) ([]DBComic, error)
}
