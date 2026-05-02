package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	wordspb "yadro.com/course/proto/words"
	"yadro.com/course/words/words"
)

const (
	maxSizeOfPhrase = 4 << 10
	grpcMaxMsgSize  = maxSizeOfPhrase + 512
)

type configPort struct {
	Port string `yaml:"port" env:"WORDS_GRPC_PORT" env-default:"8080"`
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
	wordspb.UnimplementedWordsServer
}

func (s *server) Ping(_ context.Context, in *emptypb.Empty) (*emptypb.Empty, error) {
	return nil, nil
}

func (s *server) Norm(ctx context.Context, in *wordspb.WordsRequest) (*wordspb.WordsReply, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if len(in.Phrase) > maxSizeOfPhrase {
		slog.Info("request size", "bytes", in.Phrase)
		return nil, status.Errorf(codes.ResourceExhausted, "big size of phrase, limit: %v", maxSizeOfPhrase)
	}
	words, err := words.NormalizePhrase(ctx, in.Phrase)
	if err != nil {
		st := status.FromContextError(err)
		if st.Code() == codes.DeadlineExceeded {
			slog.Warn("normalization time out", "size_message", len(in.Phrase))
		}
		if st.Code() == codes.Canceled {
			slog.Info("client cancelled request")
		}
		return nil, st.Err()
	}
	return &wordspb.WordsReply{
		Words: words,
	}, nil
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
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer(
		grpc.MaxRecvMsgSize(grpcMaxMsgSize),
	)
	wordspb.RegisterWordsServer(s, &server{})
	reflection.Register(s)

	if err := s.Serve(listener); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
