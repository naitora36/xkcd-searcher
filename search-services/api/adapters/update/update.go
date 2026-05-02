package update

import (
	"context"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"yadro.com/course/api/core"
	updatepb "yadro.com/course/proto/update"
)

type Client struct {
	log    *slog.Logger
	client updatepb.UpdateClient
	Conn   *grpc.ClientConn
}

func NewClient(address string, log *slog.Logger) (*Client, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	c := updatepb.NewUpdateClient(conn)

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

func (c Client) Status(ctx context.Context) (core.UpdateStatus, error) {
	s, err := c.client.Status(ctx, nil)
	if err != nil {
		return "", c.convertToCoreError(err)
	}

	switch s.Status {
	case updatepb.Status_STATUS_IDLE:
		return core.StatusUpdateIdle, nil
	case updatepb.Status_STATUS_RUNNING:
		return core.StatusUpdateRunning, nil
	default:
		return core.StatusUpdateUnknown, nil
	}
}

func (c Client) Stats(ctx context.Context) (core.UpdateStats, error) {
	statXKCD, err := c.client.Stats(ctx, nil)
	if err != nil {
		return core.UpdateStats{}, c.convertToCoreError(err)
	}

	return core.UpdateStats{
		WordsTotal:    int(statXKCD.WordsTotal),
		ComicsTotal:   int(statXKCD.ComicsTotal),
		WordsUnique:   int(statXKCD.WordsUnique),
		ComicsFetched: int(statXKCD.ComicsFetched),
	}, nil
}

func (c Client) Update(ctx context.Context) error {
	_, err := c.client.Update(ctx, nil)
	if err != nil {
		return c.convertToCoreError(err)
	}

	return nil
}

func (c Client) Drop(ctx context.Context) error {
	_, err := c.client.Drop(ctx, nil)
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
