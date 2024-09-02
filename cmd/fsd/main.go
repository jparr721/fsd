package main

import (
	"context"
	"fsd/internal/routes"
	"fsd/pkg/ipc"
	"fsd/pkg/tasks"
	"net/http"
	"os"
	"os/signal"
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

func main() {
	zap.L().Info("Starting up")
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		zap.L().Fatal("startup failed", zap.Error(err))
	}
	defer watcher.Close()

	rootPath := "testdir"

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
	)
	registry.Run(ctx)

	go processEventStream(ctx, watcher, broadcaster)

	err = watcher.Add(rootPath)
	if err != nil {
		zap.L().Fatal("failed to watch directory", zap.String("dirname", rootPath), zap.Error(err))
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	// Spin up the web server
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.URLFormat)
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
		Addr:    ":16000",
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
