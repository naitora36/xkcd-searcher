package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
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
	maxPhraseLen   = 4 << 10
	grpcMaxMsgSize = maxPhraseLen + 512
)

type config struct {
	Addr           string        `yaml:"words_address" env:"WORDS_ADDRESS" env-default:"localhost:80"`
	ShutdownPeriod time.Duration `yaml:"shutdown_period" env:"WORDS_GRPC_SHUTDOWN_PERIOD" env-default:"10s"`
}

func getConfig() (config, error) {
	var cfg config

	var configPath string

	flag.StringVar(&configPath, "config", "config.yaml", "path to config file")
	flag.Parse()

	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		return config{}, err
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

	if len(in.Phrase) > maxPhraseLen {
		slog.Info("request size", "bytes", in.Phrase)
		return nil, status.Errorf(codes.ResourceExhausted, "big size of phrase, limit: %v", maxPhraseLen)
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
	if err := run(); err != nil {
		slog.Error("application failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := getConfig()
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	s := grpc.NewServer(
		grpc.MaxRecvMsgSize(grpcMaxMsgSize),
	)

	ss := &server{}
	wordspb.RegisterWordsServer(s, ss)

	reflection.Register(s)

	serverDone := make(chan struct{})

	go func() {
		<-ctx.Done()

		slog.Info("shutting down gRPC server...")

		timer := time.AfterFunc(cfg.ShutdownPeriod, func() {
			slog.Warn("Server couldn't stop gracefully in time. Doing force stop.")
			s.Stop()
		})

		defer timer.Stop()

		s.GracefulStop()

		slog.Info("Server stopped gracefully.")

		close(serverDone)
	}()

	if err := s.Serve(listener); err != nil && err != grpc.ErrServerStopped {
		return fmt.Errorf("failed to serve: %w", err)
	}

	<-serverDone
	return nil
}
