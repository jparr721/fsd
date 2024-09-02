package tasks

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"fsd/pkg/ipc"
	"io/fs"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

const METADATA_DB_FILENAME string = "fsd.db"

// METADATA_CREATE creates the metadata database table
const METADATA_CREATE string = `
	CREATE TABLE IF NOT EXISTS metadata (
		id INTEGER NOT NULL PRIMARY KEY,
		full_path TEXT NOT NULL,
		size_bytes INTEGER NOT NULL,
		file_mode INTEGER NOT NULL,
		is_directory INTEGER NOT NULL,
		created_at DATETIME NOT NULL,
		modified_at DATETIME NOT NULL
	)
`

// METADATA_UPDATE_INTERVAL is how often we update the metadata for all of the files
const METADATA_UPDATE_INTERVAL = 500 * time.Millisecond

type MetadataMessage struct {
	Name      string `json:"event_name"`
	Operation string `json:"event_operation"`
}

func (m MetadataMessage) String() (string, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

func (m MetadataMessage) EventName() string {
	return m.Name
}

func (m MetadataMessage) EventOperation() string {
	return m.Operation
}

type MetadataTaskState struct {
	// rootPath is the root path that we're watching
	rootPath string

	// broadcaster is a pointer to the broadcaster.
	broadcaster *ipc.Broadcaster

	// broadcastChannel is the broadcast channel for this task.
	broadcastChannel chan ipc.Message

	// watcher is a reference to the fsnotify watcher, which we use to update our file index.
	watcher *fsnotify.Watcher

	// db is the sqlite database handle
	db *sql.DB
}

func NewMetadataTaskState(rootPath string, broadcaster *ipc.Broadcaster, broadcastChannel chan ipc.Message, watcher *fsnotify.Watcher) *MetadataTaskState {
	db, err := sql.Open("sqlite3", METADATA_DB_FILENAME)
	if err != nil {
		zap.L().Fatal("failed to open sqlite database", zap.Error(err))
	}

	_, err = db.Exec(METADATA_CREATE)
	if err != nil {
		zap.L().Fatal("failed to create metadata table", zap.Error(err))
	}

	return &MetadataTaskState{
		rootPath:         rootPath,
		broadcaster:      broadcaster,
		broadcastChannel: broadcastChannel,
		watcher:          watcher,
		db:               db,
	}
}

// RootPath returns the root path of the project.
func (mt *MetadataTaskState) RootPath() string {
	return mt.rootPath
}

// Broadcaster returns a pointer to the broadcaster.
func (mt *MetadataTaskState) Broadcaster() *ipc.Broadcaster {
	return mt.broadcaster
}

// BroadcastChannel returns the broadcast channel for this task.
func (mt *MetadataTaskState) BroadcastChannel() chan ipc.Message {
	return mt.broadcastChannel
}

type MetadataTask struct {
	state *MetadataTaskState
}

func MetadataTaskName() string {
	return "MetadataTask"
}

func NewMetadataTask(state *MetadataTaskState) *MetadataTask {
	return &MetadataTask{
		state: state,
	}
}

func (mt *MetadataTask) startMetadataUpdateTask(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			zap.L().Info("got shutdown signal, exiting", zap.String("task name", fmt.Sprintf("%s-%s", MetadataTaskName(), "UpdateTask")))
			return
		case <-time.After(METADATA_UPDATE_INTERVAL):
			zap.L().Debug("updating file metadata")
			go mt.recursivelyUpdateMetadata(ctx)
		}
	}
}

// recursivelyUpdateMetadata runs as a goroutine and starts from `mt.rootPath` and walks through all
// files, inserting their characteristics into the sqlite database.
func (mt *MetadataTask) recursivelyUpdateMetadata(ctx context.Context) {
	select {
	case <-ctx.Done():
		zap.L().Info("got shutdown signal, exiting", zap.String("task name", fmt.Sprintf("%s-%s-%s", MetadataTaskName(), "UpdateTask", "UpdateMetadataOperation")))
		return
	case <-time.After(METADATA_UPDATE_INTERVAL):
		zap.L().Error("metadata update overlap!")
	case <-time.After(2 * METADATA_UPDATE_INTERVAL):
		zap.L().Error("still delayed updating, killing task")
		return
	default:
		tx, err := mt.state.db.BeginTx(ctx, nil)
		if err != nil {
			zap.L().Error("failed to begin transaction", zap.Error(err))
			return
		}

		stmt, err := tx.Prepare(`
			INSERT INTO metadata (full_path, size_bytes, file_mode, is_directory, created_at, modified_at)
			VALUES (?, ?, ?, ?, ?, ?)
		`)
		if err != nil {
			tx.Rollback()
			zap.L().Error("failed to prepare insert statement", zap.Error(err))
		}
		defer stmt.Close()

		// Otherwise, begin updating from the walk
		err = filepath.Walk(mt.state.rootPath, func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Golang is silly...
			isDirectory := 0
			if info.IsDir() {
				isDirectory = 1
			}

			_, err = stmt.Exec(
				path,
				info.Size(),
				info.Mode().Perm(),
				isDirectory,
				time.Now(),
				info.ModTime(),
			)

			if err != nil {
				return err
			}

			return nil
		})

		if err != nil {
			zap.L().Error("metadata update task failed", zap.Error(err))
		}

		// Commit the transaction
		if err := tx.Commit(); err != nil {
			zap.L().Error("failed to commit transaction", zap.Error(err))
			return
		}

		zap.L().Debug("metadata update successful")

		return
	}
}

func (mt *MetadataTask) StartEventLoop(ctx context.Context) {
	// Start background tasks
	go mt.startMetadataUpdateTask(ctx)

	for {
		select {
		case event := <-mt.state.BroadcastChannel():
			if err := mt.HandlMessage(event); err != nil {
				zap.L().Error("error handling message", zap.String("task name", MetadataTaskName()), zap.Error(err))
			}
		case <-ctx.Done():
			zap.L().Info("got shutdown signal, exiting", zap.String("task name", MetadataTaskName()))
			return
		}
	}
}

// HandleMessage handles a network message
func (mt *MetadataTask) HandlMessage(msg ipc.Message) error {
	ms, err := msg.String()
	if err != nil {
		zap.L().Error("received invalid message", zap.Error(err))
		return err
	}
	zap.L().Debug("got message", zap.String("task name", MetadataTaskName()), zap.String("msg", ms))

	switch msg.EventOperation() {
	case ipc.Create:
		return mt.CreateMetadataEntry(msg.EventName())
	case ipc.Remove:
	case ipc.Rename:
	case ipc.Write:
	}

	return nil
}

// SendMessage sends a message over the network
func (mt *MetadataTask) SendMessage(msg ipc.Message) error {
	ms, err := msg.String()
	if err != nil {
		zap.L().Error("attempted to send an invalid message", zap.Error(err))
		return err
	}
	zap.L().Debug("sent message", zap.String("task name", MetadataTaskName()), zap.String("msg", ms))
	return nil
}

func (mt *MetadataTask) CreateMetadataEntry(name string) error {
	zap.L().Debug("Creating metadata entry", zap.String("name", name))
	// Walk the directory, adding watches for all subdirectories
	err := filepath.Walk(name, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			zap.L().Debug("adding subdirectory", zap.String("path", path))
			err = mt.state.watcher.Add(path)
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	return nil
}
