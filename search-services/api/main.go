package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"yadro.com/course/api/adapters/rest"
	"yadro.com/course/api/adapters/words"
	"yadro.com/course/api/config"
	"yadro.com/course/api/core"
)

func main() {
	var configPath string

	flag.StringVar(&configPath, "config", "config.yaml", "server configuration file")
	flag.Parse()

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		slog.Error("error when read config", "error", err)
		os.Exit(1)
	}

	log, err := makeLogger(cfg.LogLevel)
	if err != nil {
		slog.Error("failed to make logger", "error", err)
		os.Exit(1)
	}

	wordsClient, err := words.NewClient(cfg.WordsAddress, log)
	if err != nil {
		log.Error("cannot init words adapter", "error", err)
		os.Exit(1)
	}
	defer func() {
		err := wordsClient.Close()
		if err != nil {
			log.Error("close client error", "error", err)
		}
	}()

	mux := http.NewServeMux()
	mux.Handle("GET /api/words", rest.NewWordsHandler(log, wordsClient))
	mux.Handle("GET /ping", rest.NewPingHandler(log, map[string]core.Pinger{"words": wordsClient}))

	server := http.Server{
		Addr:        cfg.HTTPServer.Address,
		ReadTimeout: cfg.HTTPServer.ReadTimeout,
		Handler:     mux,
	}

	serverDone := make(chan struct{})
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()

		log.Info("shutting down http server...")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTPServer.ShutdownPeriod)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Error("erroneous shutdown", "error", err)
		}

		close(serverDone)
	}()

	if err := server.ListenAndServe(); err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			log.Error("server closed unexpectedly", "error", err)
			return
		}
	}
	<-serverDone
}

func makeLogger(logLevel string) (*slog.Logger, error) {
	var level slog.Level

	err := level.UnmarshalText([]byte(logLevel))
	if err != nil {
		return nil, err
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	handler := slog.NewTextHandler(os.Stdout, opts)

	return slog.New(handler), nil
}
