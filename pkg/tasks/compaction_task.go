package tasks

import (
	"context"
	"database/sql"
	"fsd/pkg/ipc"
	"time"

	"go.uber.org/zap"
)

// COMPACTION_INTERVAL is the time between compaction runs
const COMPACTION_INTERVAL = 1 * time.Minute

type CompactionTaskState struct {
	// rootPath is the root path that we're watching
	rootPath string

	// broadcaster is a pointer to the broadcaster.
	broadcaster *ipc.Broadcaster

	// broadcastChannel is the broadcast channel for this task.
	broadcastChannel chan ipc.Message

	// db is the sqlite database handle
	db *sql.DB
}

func NewCompactionTaskState(rootPath string, broadcaster *ipc.Broadcaster, broadcastChannel chan ipc.Message) *CompactionTaskState {
	db, err := sql.Open("sqlite3", METADATA_DB_FILENAME)
	if err != nil {
		zap.L().Fatal("failed to open sqlite database", zap.Error(err))
	}

	_, err = db.Exec(METADATA_CREATE)
	if err != nil {
		zap.L().Fatal("failed to create metadata table", zap.Error(err))
	}

	return &CompactionTaskState{
		rootPath:         rootPath,
		broadcaster:      broadcaster,
		broadcastChannel: broadcastChannel,
		db:               db,
	}
}

// RootPath returns the root path of the project.
func (ct *CompactionTaskState) RootPath() string {
	return ct.rootPath
}

// Broadcaster returns a pointer to the broadcaster.
func (ct *CompactionTaskState) Broadcaster() *ipc.Broadcaster {
	return ct.broadcaster
}

// BroadcastChannel returns the broadcast channel for this task.
func (ct *CompactionTaskState) BroadcastChannel() chan ipc.Message {
	return ct.broadcastChannel
}

// CompactionTask spins up on a regular interval and culls records older than
// the configured point. This is not real compaction per-se... yet.
type CompactionTask struct {
	state *CompactionTaskState
}

func CompactionTaskName() string {
	return "CompactionTask"
}

func NewCompactionTask(state *CompactionTaskState) *CompactionTask {
	return &CompactionTask{
		state: state,
	}
}

// compactStaleRecords deletes stale records. This is *not* compaction as is found in
// systems like [rocksdb](https://github.com/facebook/rocksdb/wiki/Compaction), but
// it eventually will support more comprehensive operations once the need arises.
func (ct *CompactionTask) compactStaleRecords(ctx context.Context) {
	thresh := time.Now().Add(-METADATA_UPDATE_INTERVAL)
	query := `
		DELETE FROM metadata WHERE created_at < ?
	`

	// Nuke the old data
	result, err := ct.state.db.ExecContext(ctx, query, thresh)
	if err != nil {
		zap.L().Error("compaction operation failed", zap.Error(err))
		return
	}

	// Report how many rows were affected
	rowsDeleted, err := result.RowsAffected()
	if err != nil {
		zap.L().Error("failed to recover changed rows", zap.Error(err))
	} else {
		zap.L().Info("deleted old records", zap.Int64("rows deleted", rowsDeleted))
	}
}

func (ct *CompactionTask) StartEventLoop(ctx context.Context) {
	for {
		select {
		case event := <-ct.state.BroadcastChannel():
			if err := ct.HandlMessage(event); err != nil {
				zap.L().Error("error handling message", zap.String("task name", CompactionTaskName()), zap.Error(err))
			}
		case <-time.After(COMPACTION_INTERVAL):
			zap.L().Info("beginning compaction operation")
			// We do this as a blocking operation since it's already off the main thread.
			ct.compactStaleRecords(ctx)
		case <-ctx.Done():
			zap.L().Info("got shutdown signal, exiting", zap.String("task name", CompactionTaskName()))
			return
		}
	}
}

// HandleMessage handles a network message
func (ct *CompactionTask) HandlMessage(msg ipc.Message) error {
	ms, err := msg.String()
	if err != nil {
		zap.L().Error("received invalid message", zap.Error(err))
		return err
	}
	zap.L().Debug("got message", zap.String("task name", CompactionTaskName()), zap.String("msg", ms))

	return nil
}

// SendMessage sends a message over the network
func (ct *CompactionTask) SendMessage(msg ipc.Message) error {
	ms, err := msg.String()
	if err != nil {
		zap.L().Error("attempted to send an invalid message", zap.Error(err))
		return err
	}
	zap.L().Debug("sent message", zap.String("task name", CompactionTaskName()), zap.String("msg", ms))
	return nil
}
