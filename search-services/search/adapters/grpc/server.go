package grpc

import (
	"context"
	"log/slog"

	"google.golang.org/protobuf/types/known/emptypb"
	searchpb "yadro.com/course/proto/search"
	"yadro.com/course/search/core"
)

type Server struct {
	searchpb.UnimplementedSearchServer
	service core.Searcher
}

func NewServer(service core.Searcher) *Server {
	return &Server{
		service: service,
	}
}

func (s Server) Ping(_ context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	return nil, nil
}

func (s Server) Search(ctx context.Context, in *searchpb.SearchRequest) (*searchpb.SearchReply, error) {
	reply, err := s.service.Search(ctx, &core.SearchRequest{
		Phrase: in.Phrase,
		Limit:  int(in.Limit),
	})
	if err != nil {
		slog.Error("service search error", "error", err)
		return nil, err
	}

	pbComics := make([]*searchpb.Comic, 0, len(reply.Comics))

	for _, comic := range reply.Comics {
		pbComics = append(pbComics, &searchpb.Comic{
			Id:  int64(comic.ID),
			Url: comic.URL,
		})
	}

	return &searchpb.SearchReply{
		Comics: pbComics,
	}, nil
}
