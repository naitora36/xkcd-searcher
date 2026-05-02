package search

import (
	"context"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"yadro.com/course/api/core"
	searchpb "yadro.com/course/proto/search"
)

type Client struct {
	log    *slog.Logger
	client searchpb.SearchClient
	Conn   *grpc.ClientConn
}

func NewClient(address string, log *slog.Logger) (*Client, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	c := searchpb.NewSearchClient(conn)

	return &Client{
		log:    log,
		client: c,
		Conn:   conn,
	}, nil
}

func (c Client) Ping(ctx context.Context) error {
	_, err := c.client.Ping(ctx, nil)
	if err != nil {
		return c.convertToCoreError(err)
	}

	return nil
}

func (c Client) Search(ctx context.Context, phrase string, limit int) ([]core.Comics, error) {
	in := &searchpb.SearchRequest{
		Phrase: phrase,
		Limit:  int64(limit),
	}

	searchReply, err := c.client.Search(ctx, in)
	if err != nil {
		return nil, c.convertToCoreError(err)
	}

	grpcComics := searchReply.Comics

	res := make([]core.Comics, 0, len(grpcComics))
	for _, gComic := range grpcComics {
		res = append(res, core.Comics{
			ID:  int(gComic.Id),
			URL: gComic.Url,
		})
	}

	return res, nil
}

func (c Client) convertToCoreError(err error) error {
	s, ok := status.FromError(err)
	if !ok {
		return core.ErrInternal
	}

	switch s.Code() {
	case codes.Aborted:
		return core.ErrAlreadyRunning
	case codes.ResourceExhausted:
		return core.ErrResourceExhausted
	case codes.Canceled:
		return core.ErrCanceled
	case codes.DeadlineExceeded:
		return core.ErrDeadlineExceeded
	default:
		c.log.Error("unexpected grpc error", "code", s.Code(), "msg", s.Message())
		return core.ErrInternal
	}
}
