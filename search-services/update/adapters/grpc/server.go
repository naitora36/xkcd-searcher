package grpc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	updatepb "yadro.com/course/proto/update"
	"yadro.com/course/update/core"
)

type Server struct {
	updatepb.UnimplementedUpdateServer
	service core.Updater
}

func NewServer(service core.Updater) *Server {
	return &Server{
		service: service,
	}
}

func (s *Server) Ping(_ context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	return nil, nil
}

func (s *Server) Status(ctx context.Context, _ *emptypb.Empty) (*updatepb.StatusReply, error) {
	var statusId updatepb.Status
	status := s.service.Status(ctx)

	if status == core.StatusIdle {
		statusId = updatepb.Status_STATUS_IDLE
	}
	if status == core.StatusRunning {
		statusId = updatepb.Status_STATUS_RUNNING
	}

	res := &updatepb.StatusReply{
		Status: statusId,
	}
	return res, nil
}

func (s *Server) Update(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	err := s.service.Update(ctx)
	if err != nil {
		if errors.Is(err, core.ErrAlreadyRunning) {
			return nil, status.Error(codes.Aborted, err.Error())
		}
		return &emptypb.Empty{}, err
	}

	return &emptypb.Empty{}, nil
}

func (s *Server) Stats(ctx context.Context, _ *emptypb.Empty) (*updatepb.StatsReply, error) {
	stats, err := s.service.Stats(ctx)
	if err != nil {
		return &updatepb.StatsReply{}, err
	}
	return &updatepb.StatsReply{
		WordsTotal:    int64(stats.WordsTotal),
		WordsUnique:   int64(stats.WordsUnique),
		ComicsTotal:   int64(stats.ComicsTotal),
		ComicsFetched: int64(stats.ComicsFetched),
	}, nil
}

func (s *Server) Drop(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	err := s.service.Drop(ctx)
	if err != nil {
		return &emptypb.Empty{}, err
	}
	return &emptypb.Empty{}, nil
}
