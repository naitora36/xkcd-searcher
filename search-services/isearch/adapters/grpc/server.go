package grpc

import (
	"context"
	"log/slog"

	"google.golang.org/protobuf/types/known/emptypb"
	"yadro.com/course/isearch/core"
	searchpb "yadro.com/course/proto/isearch"
)

type Server struct {
	searchpb.UnimplementedISearchServer
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

func (s Server) SearchIndex(ctx context.Context, in *searchpb.SearchRequest) (*searchpb.SearchReply, error) {
	reply, err := s.service.SearchIndex(ctx, &core.SearchRequest{
		Phrase: in.Phrase,
		Limit:  int(in.Limit),
	})
	if err != nil {
		slog.Error("service search_index error", "error", err)
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
