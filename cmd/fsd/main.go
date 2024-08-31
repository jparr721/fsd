package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

func init() {
	// TODO: Change this later.
	logger := zap.Must(zap.NewDevelopment())
	zap.ReplaceGlobals(logger)
}

func processEventStream(ctx context.Context, watcher *fsnotify.Watcher) {
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				zap.L().Error("Failed to receive watcher event")
				return
			}

			zap.L().Info("Received event", zap.String("event", event.String()))
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

	ctx, cancel := context.WithCancel(context.Background())
	go processEventStream(ctx, watcher)

	err = watcher.Add("testdir/foo")
	if err != nil {
		zap.L().Fatal("failed to watch directory", zap.String("dirname", "testdir"), zap.Error(err))
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	// Handle the shutdown signals, blocking until they're received
	select {
	case <-signalChan:
		zap.L().Info("Received shutdown signal")
		// Cancel background threads
		cancel()
	}

	zap.L().Info("Shutting down")
}
