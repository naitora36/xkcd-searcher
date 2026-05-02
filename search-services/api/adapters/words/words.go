package words

import (
	"context"
	"fmt"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"yadro.com/course/api/core"
	wordspb "yadro.com/course/proto/words"
)

type Client struct {
	log    *slog.Logger
	client wordspb.WordsClient
	Conn   *grpc.ClientConn
}

func NewClient(address string, log *slog.Logger) (*Client, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	c := wordspb.NewWordsClient(conn)

	return &Client{
		log:    log,
		client: c,
		Conn:   conn,
	}, nil
}

func (c Client) Norm(ctx context.Context, phrase string) ([]string, error) {
	if phrase == "" {
		return nil, core.ErrEmptyPhrase
	}

	wordsReq := &wordspb.WordsRequest{
		Phrase: phrase,
	}

	wordsReply, err := c.client.Norm(ctx, wordsReq)
	if err != nil {
		return nil, c.convertToCoreError(err)
	}

	return wordsReply.GetWords(), nil
}

func (c Client) Ping(ctx context.Context) error {
	_, err := c.client.Ping(ctx, nil)
	if err != nil {
		return c.convertToCoreError(err)
	}

	return nil
}

func (c Client) convertToCoreError(err error) error {
	s, ok := status.FromError(err)
	if !ok {
		return core.ErrInternal
	}

	switch s.Code() {
	case codes.ResourceExhausted:
		return core.ErrResourceExhausted
	case codes.Canceled:
		return core.ErrCanceled
	case codes.DeadlineExceeded:
		return core.ErrDeadlineExceeded
	default:
		return fmt.Errorf("unexpected norm error: %w", err)
	}
}
