package tasks

import (
	"context"
	"database/sql"
	"encoding/json"
	"fsd/ext/du"
	"fsd/pkg/ipc"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

// DISK_STATS_CREATE creates the disk_stats table for storing disk statistics about the
// root path for the system.
const DISK_STATS_CREATE string = `
	CREATE TABLE IF NOT EXISTS disk_stats(
		id INTEGER NOT NULL PRIMARY KEY,
		free INTEGER NOT NULL,
		available INTEGER NOT NULL,
		size INTEGER NOT NULL,
		used INTEGER NOT NULL,
		used_pct FLOAT NOT NULL,
		created_at DATETIME NOT NULL
	)
`

type FsMessage struct {
	Name      string    `json:"event_name"`
	Operation ipc.FsdOp `json:"event_operation"`
}

func NewFromINotifyEvent(iNotifyEvent fsnotify.Event) FsMessage {
	return FsMessage{
		Name:      iNotifyEvent.Name,
		Operation: ipc.NewFsdOpFromINotifyOp(iNotifyEvent.Op),
	}
}

func (fs FsMessage) String() (string, error) {
	b, err := json.Marshal(fs)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

func (fs FsMessage) EventName() string {
	return fs.Name
}

func (fs FsMessage) EventOperation() ipc.FsdOp {
	return fs.Operation
}

type FsTaskState struct {
	// rootPath is the root path of the project.
	rootPath string

	// broadcaster is a pointer to the broadcaster.
	broadcaster *ipc.Broadcaster

	// broadcastChannel is the broadcast channel for this task.
	broadcastChannel chan ipc.Message

	// db is the sqlite database handle
	db *sql.DB
}

func NewFsTaskState(rootPath string, broadcaster *ipc.Broadcaster, broadcastChannel chan ipc.Message) *FsTaskState {
	db, err := sql.Open("sqlite3", FSD_DB_FILENAME)
	if err != nil {
		zap.L().Fatal("failed to open sqlite database", zap.Error(err))
	}

	_, err = db.Exec(DISK_STATS_CREATE)
	if err != nil {
		zap.L().Fatal("failed to create disk_stats table", zap.Error(err))
	}

	return &FsTaskState{
		rootPath:         rootPath,
		broadcaster:      broadcaster,
		broadcastChannel: broadcastChannel,
		db:               db,
	}
}

// RootPath returns the root path of the project.
func (fs *FsTaskState) RootPath() string {
	return fs.rootPath
}

// Broadcaster returns a pointer to the broadcaster.
func (fs *FsTaskState) Broadcaster() *ipc.Broadcaster {
	return fs.broadcaster
}

// BroadcastChannel returns the broadcast channel for this task.
func (fs *FsTaskState) BroadcastChannel() chan ipc.Message {
	return fs.broadcastChannel
}

type FsTask struct {
	// state is the state of the Fs Task
	state *FsTaskState
}

func NewFsTask(state *FsTaskState) *FsTask {
	return &FsTask{
		state: state,
	}
}

func FsTaskName() string {
	return "FsTask"
}

func (fs *FsTask) doCompaction(ctx context.Context) error {
	// Keep the most recent 5 elements
	// TODO: Make this configurable
	query := `
		DELETE FROM disk_stats
		WHERE id NOT IN (
			SELECT id FROM disk_stats
			ORDER BY created_at DESC
			LIMIT 5
		);
	`

	// Nuke the old data
	result, err := fs.state.db.ExecContext(ctx, query)
	if err != nil {
		return err
	}

	// Report how many rows were affected
	rowsDeleted, err := result.RowsAffected()
	if err != nil {
		return err
	}

	zap.L().Info("deleted old records", zap.String("table name", "disk_stats"), zap.Int64("rows deleted", rowsDeleted))
	return nil
}

func (fs *FsTask) StartEventLoop(ctx context.Context) {
	// First startup, compute disk stats
	if err := fs.RecomputeDiskStatistics(ctx); err != nil {
		zap.L().Fatal("failed to compute initial disk stats for root dir")
	}

	for {
		select {
		case event := <-fs.state.BroadcastChannel():
			if err := fs.HandleMessage(ctx, event); err != nil {
				zap.L().Error("error handling message", zap.String("task name", FsTaskName()), zap.Error(err))
			}
		case <-time.After(5 * time.Second):
			if err := fs.RecomputeDiskStatistics(ctx); err != nil {
				zap.L().Error("error computing disk stats", zap.String("task name", FsTaskName()), zap.Error(err))
			}
		case <-ctx.Done():
			zap.L().Info("got shutdown signal, exiting", zap.String("task name", FsTaskName()))
			return
		}
	}
}

// HandleMessage handles a network message
func (fs *FsTask) HandleMessage(ctx context.Context, msg ipc.Message) error {
	// if it's a compact message, compact!
	if msg.EventOperation() == ipc.Compact {
		return fs.doCompaction(ctx)
	}

	// otherwise get the disk stats
	return fs.RecomputeDiskStatistics(ctx)
}

// SendMessage sends a message over the network
func (fs *FsTask) SendMessage(msg ipc.Message) error {
	ms, err := msg.String()
	if err != nil {
		zap.L().Error("Attempted to send an invalid message", zap.Error(err))
		return err
	}
	zap.L().Info("sent message", zap.String("task name", FsTaskName()), zap.String("msg", ms))
	return nil
}

func (fs *FsTask) RecomputeDiskStatistics(ctx context.Context) error {
	du := du.NewDiskUsage(fs.state.RootPath())

	// Prepare the SQL statement
	stmt, err := fs.state.db.Prepare(`
		INSERT INTO disk_stats(free, available, size, used, used_pct, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		zap.L().Error("failed to prepare insert statement", zap.String("task name", FsTaskName()), zap.Error(err))
		return err
	}
	defer stmt.Close()

	// Execute the SQL statement
	_, err = stmt.Exec(
		du.Free(),
		du.Available(),
		du.Size(),
		du.Used(),
		du.Usage(),
		time.Now(),
	)

	if err != nil {
		zap.L().Error("failed to insert into disk_stats", zap.Error(err))
		return err
	}

	return nil
}
