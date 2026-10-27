package main

import (
	"context"
	"flag"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nightgopher/cc4-journey/services/lead-source-mock/internal/simulator"
)

var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print build revision")
	flag.Parse()
	if *showVersion {
		println(version)
		return
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	cfg, err := simulator.FromEnvironment()
	if err != nil {
		slog.Error("configuration rejected", "error", err)
		os.Exit(1)
	}
	handler, err := simulator.New(cfg, version)
	if err != nil {
		slog.Error("configuration rejected", "error", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	server := &http.Server{Addr: net.JoinHostPort(env("HOST", "127.0.0.1"), env("PORT", "18081")), Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 35 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		slog.Error("listen failed", "error", err)
		os.Exit(1)
	}
	slog.Info("mock lead source listening", "address", listener.Addr().String(), "revision", version)
	if err = serve(ctx, server, listener); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func serve(ctx context.Context, server *http.Server, listener net.Listener) error {
	ctx, stop := context.WithCancel(ctx)
	defer stop()
	drained := make(chan error, 1)
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 35*time.Second)
		defer cancel()
		err := server.Shutdown(shutdown)
		if err != nil {
			_ = server.Close()
		}
		drained <- err
	}()
	err := server.Serve(listener)
	stop()
	shutdownErr := <-drained
	if err != http.ErrServerClosed {
		return err
	}
	return shutdownErr
}

func env(key, fallback string) string {
	if s := os.Getenv(key); s != "" {
		return s
	}
	return fallback
}
