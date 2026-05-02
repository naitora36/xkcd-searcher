package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"

	"github.com/VictoriaMetrics/metrics"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"yadro.com/course/closers"
	"yadro.com/course/isearch/adapters/broker"
	"yadro.com/course/isearch/adapters/db"
	isearch "yadro.com/course/isearch/adapters/grpc"
	"yadro.com/course/isearch/adapters/initiator"
	"yadro.com/course/isearch/adapters/words"
	"yadro.com/course/isearch/config"
	"yadro.com/course/isearch/core"
	searchpb "yadro.com/course/proto/isearch"
)

func main() {
	// config
	var configPath string
	flag.StringVar(&configPath, "config", "config.yaml", "server configuration file")
	flag.Parse()
	cfg := config.MustLoad(configPath)

	// logger
	log := mustMakeLogger(cfg.LogLevel)

	if err := run(cfg, log); err != nil {
		log.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func run(cfg config.Config, log *slog.Logger) error {
	log.Info("starting server")
	log.Debug("debug messages are enabled")

	// database adapter
	storage, err := db.New(log, cfg.DBAddress)
	if err != nil {
		return fmt.Errorf("failed to connect to db: %v", err)
	}
	defer closers.CloseOrLog(storage.Conn, log)

	// words adapter
	words, err := words.NewClient(cfg.WordsAddress, log)
	if err != nil {
		return fmt.Errorf("failed create Words client: %v", err)
	}
	defer closers.CloseOrLog(words.Conn, log)
	// service
	searcher, err := core.NewService(log, storage, words)
	if err != nil {
		return fmt.Errorf("failed create Update service: %v", err)
	}

	// grpc server
	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	searchpb.RegisterISearchServer(s, isearch.NewServer(searcher))
	reflection.Register(s)

	// context for Ctrl-C
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// initiator
	indexTicker := initiator.NewInitiator(cfg.TTL, searcher)

	go func() {
		indexTicker.Tick(ctx, log)
	}()

	defer indexTicker.StopTicker()
	// broker
	broker, err := broker.NewBroker(cfg.BrokerAddress, searcher, log)
	if err != nil {
		return fmt.Errorf("failed to create broker subscriber: %v", err)
	}
	defer closers.CloseOrLog(broker, log)

	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, req *http.Request) {
		metrics.WritePrometheus(w, true)
	})

	metricsServer := &http.Server{
		Addr:    cfg.MetricsServerAddress,
		Handler: mux,
	}

	go func() {
		log.Info("Starting metrics HTTP server on", "address", cfg.MetricsServerAddress)
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("Metrics server failed", "error", err)
		}
	}()

	// проверяем появились ли новые ивенты
	go func() {
		err := broker.GetEvent(ctx)
		if err != nil {
			log.Error("error when get event", "error", err)
		}
	}()

	go func() {
		<-ctx.Done()
		log.Debug("shutting down server")
		s.GracefulStop()
		if err := metricsServer.Shutdown(context.Background()); err != nil {
			log.Error("erroneous shutdown", "error", err)
		}
	}()

	if err := s.Serve(listener); err != nil {
		return fmt.Errorf("failed to serve: %v", err)
	}
	return nil
}

func mustMakeLogger(logLevel string) *slog.Logger {
	var level slog.Level
	switch logLevel {
	case "DEBUG":
		level = slog.LevelDebug
	case "INFO":
		level = slog.LevelInfo
	case "ERROR":
		level = slog.LevelError
	default:
		panic("unknown log level: " + logLevel)
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})

	return slog.New(handler)
}
