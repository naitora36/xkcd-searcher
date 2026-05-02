package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	petname "github.com/dustinkirkland/golang-petname"
	"github.com/ilyakaznacheev/cleanenv"
	petnamepb "yadro.com/course/proto"
)

const (
	maxWords = 1000
	maxNames = 1_000_000
)

type configPort struct {
	Port string `yaml:"port" env:"PETNAME_GRPC_PORT" env-default:"8080"`
}

func getConfigPort() (configPort, error) {
	var cfg configPort

	var configPath string

	flag.StringVar(&configPath, "config", "", "path to config file")
	flag.Parse()

	if configPath != "" {
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			return configPort{}, fmt.Errorf("config file not found: %v", configPath)
		}
		if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
			return configPort{}, fmt.Errorf("failed to read config: %w", err)
		}
	} else {
		if err := cleanenv.ReadEnv(&cfg); err != nil {
			return configPort{}, fmt.Errorf("failed to read env: %w", err)
		}
	}
	return cfg, nil
}

type server struct {
	petnamepb.UnimplementedPetnameGeneratorServer
}

func (s *server) Ping(_ context.Context, in *emptypb.Empty) (*emptypb.Empty, error) {
	return nil, nil
}

func (s *server) Generate(_ context.Context, req *petnamepb.PetnameRequest) (*petnamepb.PetnameResponse, error) {
	words := req.Words
	if words <= 0 {
		slog.Info("invalid request parameters",
			"words", words,
			"error_code", codes.InvalidArgument.String(),
		)
		return nil, status.Error(codes.InvalidArgument, "variable words must be positive")
	}
	if words > maxWords {
		return nil, status.Errorf(codes.InvalidArgument, "too many words: max is %d", maxWords)
	}

	name := petname.Generate(int(words), req.Separator)
	return &petnamepb.PetnameResponse{
		Name: name,
	}, nil
}

func (s *server) GenerateMany(req *petnamepb.PetnameStreamRequest, stream grpc.ServerStreamingServer[petnamepb.PetnameResponse]) error {
	words := req.Words
	names := req.Names
	if words <= 0 || names <= 0 {
		slog.Info("invalid request parameters",
			"words", words,
			"names", names,
			"error_code", codes.InvalidArgument.String(),
		)
		return status.Error(codes.InvalidArgument, "variable words and names must be positive")
	}
	if words > maxWords {
		return status.Errorf(codes.InvalidArgument, "too many words: max is %d", maxWords)
	}
	if names > maxNames {
		return status.Errorf(codes.InvalidArgument, "too many names: max is %d", maxNames)
	}
	for range names {
		name := petname.Generate(int(words), req.Separator)

		resp := petnamepb.PetnameResponse{
			Name: name,
		}
		err := stream.Send(&resp)
		if err != nil {
			st, ok := status.FromError(err)
			if ok {
				switch st.Code() {
				case codes.Canceled, codes.DeadlineExceeded, codes.Unavailable:
					slog.Info("client disconnected",
						"code", st.Code().String(),
						"description", st.Message(),
					)
					return err
				}
			}
			slog.Error("stream send error", "error", err)
			return status.Error(codes.Internal, "failed to send response")
		}
	}
	return nil
}

func main() {
	cfg, err := getConfigPort()
	if err != nil {
		slog.Error("config error", "error", err)
		os.Exit(1)
	}
	addr := ":" + cfg.Port

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("failed to listen", "error", err)
		os.Exit(1)
	}

	s := grpc.NewServer()
	petnamepb.RegisterPetnameGeneratorServer(s, &server{})
	reflection.Register(s)

	if err := s.Serve(listener); err != nil {
		slog.Error("failed to serve", "error", err)
		os.Exit(1)
	}
}
