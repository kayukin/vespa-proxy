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
	"syscall"
	"time"

	"vespa-proxy/internal/config"
	"vespa-proxy/internal/proxy"
	"vespa-proxy/internal/ui"

	"github.com/jchv/go-webview-selector"
)

func main() {
	configPath := flag.String("config", "", "path to YAML config file (default: $CONFIG_FILE or config.yaml)")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	slog.Info("starting vespa-proxy",
		"addr", cfg.ListenAddr,
		"vespa_url", cfg.VespaURL,
	)

	vespaProxy, err := proxy.New(cfg, logger)
	if err != nil {
		slog.Error("failed to create proxy", "error", err)
		os.Exit(1)
	}

	staticFS, err := ui.StaticFS()
	if err != nil {
		slog.Error("failed to load embedded UI", "error", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()

	// Proxy API requests to Vespa
	mux.Handle("/api/", http.StripPrefix("/api", vespaProxy))
	mux.Handle("/search/", vespaProxy)
	mux.Handle("/document/", vespaProxy)

	// Serve embedded static UI for all other routes
	mux.Handle("/", ui.SpaHandler(staticFS))

	server := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      loggingMiddleware(mux, logger),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("server listening", "addr", cfg.ListenAddr)
		serverErr <- server.ListenAndServe()
	}()

	w := webview.New(false)
	if w == nil {
		slog.Error("Failed to create webview")
		os.Exit(1)
	}
	w.SetTitle("Vespa Proxy")
	w.SetSize(1280, 720, webview.HintNone)
	w.Navigate(cfg.ListenAddr)
	defer w.Destroy()
	w.Run()

	slog.Info("window closed, shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err = server.Shutdown(ctx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
	}

	if err = <-serverErr; !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped")
}

// loggingMiddleware logs every request.
func loggingMiddleware(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.status,
			"duration_ms", fmt.Sprintf("%.2f", float64(time.Since(start).Microseconds())/1000),
			"remote_addr", r.RemoteAddr,
		)
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}
