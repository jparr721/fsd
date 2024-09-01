package main

import (
	"context"
	"fsd/pkg/ipc"
	"fsd/pkg/tasks"
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
	registry.Init(rootPath, broadcaster, watcher, tasks.FsTaskName(), tasks.MetadataTaskName())
	registry.Run(ctx)

	go processEventStream(ctx, watcher, broadcaster)

	err = watcher.Add(rootPath)
	if err != nil {
		zap.L().Fatal("failed to watch directory", zap.String("dirname", rootPath), zap.Error(err))
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
