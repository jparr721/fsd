package main

import (
	"context"
	"database/sql"
	"flag"
	"fsd/internal/config"
	"fsd/internal/routes"
	"fsd/pkg/ipc"
	"fsd/pkg/tasks"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/render"
	"go.uber.org/zap"
)

func init() {
	// TODO: Change this later.
	logger := zap.Must(zap.NewDevelopment())
	zap.ReplaceGlobals(logger)

	config.InitConfig()
}

func processEventStream(ctx context.Context, watcher *fsnotify.Watcher, broadcaster *ipc.Broadcaster) {
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				zap.L().Error("Failed to receive watcher event")
				return
			}

			zap.L().Info("Received event", zap.String("event", event.String()))
			broadcaster.Broadcast(tasks.NewFromINotifyEvent(event))
		case err, ok := <-watcher.Errors:
			if !ok {
				zap.L().Error("Failed to receive watcher error")
				return
			}
			zap.L().Error("Watcher error", zap.Error(err))
		case <-ctx.Done():
			zap.L().Info("Received shutdown signal, exiting")
			return
		}
	}
}

func logger(l *zap.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			t1 := time.Now()
			defer func() {
				l.Info("Served",
					zap.String("proto", r.Proto),
					zap.String("path", r.URL.Path),
					zap.Duration("lat", time.Since(t1)),
					zap.Int("status", ww.Status()),
					zap.Int("size", ww.BytesWritten()),
					zap.String("reqId", middleware.GetReqID(r.Context())))
			}()

			next.ServeHTTP(ww, r)
		}
		return http.HandlerFunc(fn)
	}
}

func dbContext(db *sql.DB) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), "db", db)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func runApp() {
	zap.L().Info("Starting up")
	zap.L().Debug("config", zap.Any("config", config.GetConfig()))
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		zap.L().Fatal("startup failed", zap.Error(err))
	}
	defer watcher.Close()

	rootPath := config.GetConfig().WatchDir

	// If `rootPath` is not absolute, make it absolute
	if !filepath.IsAbs(rootPath) {
		rootPath, err = filepath.Abs(rootPath)
		if err != nil {
			zap.L().Fatal("failed to get absolute path for root path", zap.String("path", rootPath), zap.Error(err))
		}
	}

	// If `rootPath` does not exist, create it
	if _, err := os.Stat(rootPath); os.IsNotExist(err) {
		zap.L().Info("root path does not exist, creating", zap.String("path", rootPath))
		if err := os.MkdirAll(rootPath, 0755); err != nil {
			zap.L().Fatal("failed to create root directory", zap.String("path", rootPath), zap.Error(err))
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Set up background threads
	broadcaster := ipc.NewBroadcaster()
	registry := tasks.NewTaskRegistry()
	registry.Init(
		rootPath,
		broadcaster,
		watcher,
		tasks.FsTaskName(),
		tasks.MetadataTaskName(),
		tasks.CompactionTaskName(),
		tasks.ProcTaskName(),
	)
	registry.Run(ctx)

	go processEventStream(ctx, watcher, broadcaster)

	err = watcher.Add(rootPath)
	if err != nil {
		zap.L().Fatal("failed to watch directory", zap.String("dirname", rootPath), zap.Error(err))
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	db, err := sql.Open("sqlite3", config.GetDBPath())
	if err != nil {
		zap.L().Fatal("failed to open database", zap.Error(err))
	}
	defer db.Close()

	// Spin up the web server
	r := chi.NewRouter()
	r.Use(dbContext(db))
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.URLFormat)
	r.Use(logger(zap.L()))
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "x-auth-token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))
	r.Route("/", routes.MakeRouter)

	httpServer := &http.Server{
		Addr:    config.GetConfig().ListenAddr,
		Handler: r,
	}

	go func() {
		zap.L().Info("http server running", zap.String("addr", httpServer.Addr))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zap.L().Fatal("HTTP server failed to start", zap.Error(err))
		}
	}()

	// Handle the shutdown signals, blocking until they're received
	select {
	case <-signalChan:
		zap.L().Info("Received shutdown signal")
		// Cancel background threads
		cancel()

		// Shutdown the web server
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			zap.L().Error("HTTP server shutdown failed", zap.Error(err))
		} else {
			zap.L().Info("HTTP server shutdown successfully")
		}
	}

	zap.L().Info("Shutting down")
}

func main() {
	// Parse command-line flags
	metadataUpdateInterval := flag.Duration("metadata-update-interval", config.GetConfig().MetadataUpdateInterval, "Metadata update interval")
	compactionInterval := flag.Duration("compaction-interval", config.GetConfig().CompactionInterval, "Compaction interval")
	broadcastBufferDepth := flag.Int("broadcast-buffer-depth", config.GetConfig().BroadcastBufferDepth, "Broadcast buffer depth")
	listenAddr := flag.String("listen-addr", config.GetConfig().ListenAddr, "Listen address")
	watchDir := flag.String("watch-dir", config.GetConfig().WatchDir, "Watch directory")
	flag.Parse()

	// Update config with flag values
	cfg := config.GetConfig()
	cfg.MetadataUpdateInterval = *metadataUpdateInterval
	cfg.CompactionInterval = *compactionInterval
	cfg.BroadcastBufferDepth = *broadcastBufferDepth
	cfg.ListenAddr = *listenAddr
	cfg.WatchDir = *watchDir

	runApp()
}
