package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gray/elsa-quiz/internal/handler"
	"github.com/gray/elsa-quiz/internal/store"
)

func main() {
	health := flag.Bool("health", false, "probe /api/health on $PORT and exit (for container HEALTHCHECK)")
	flag.Parse()

	port := envOr("PORT", "8080")
	if *health {
		os.Exit(runHealthCheck(port))
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	basePoints, _ := strconv.Atoi(envOr("BASE_POINTS", "10"))

	api := handler.NewAPI(store.NewMemoryStore(), basePoints)
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("quiz server listening", "addr", srv.Addr, "basePoints", basePoints)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logger.Info("shutting down")
	api.Shutdown()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown failed", "err", err)
	}
}

func runHealthCheck(port string) int {
	resp, err := http.Get(fmt.Sprintf("http://localhost:%s/api/health", port))
	if err != nil {
		return 1
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
