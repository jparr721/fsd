package tasks

import (
	"context"
	"database/sql"
	"fsd/pkg/ipc"
	"strings"
	"time"

	"go.uber.org/zap"
)

const PROC_CREATE string = `
	CREATE TABLE IF NOT EXISTS proc (
		id INTEGER NOT NULL PRIMARY KEY,
		command TEXT NOT NULL,
		args TEXT NOT NULL,
		is_executed INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL
	)
`

const PROC_RESULTS_CREATE string = `
	CREATE TABLE IF NOT EXISTS proc_results (
		id INTEGER NOT NULL PRIMARY KEY,
		stdout TEXT NOT NULL,
		stderr TEXT NOT NULL,
		created_at DATETIME NOT NULL
	)
`

// ProcTaskState is the state for the proc task.
type ProcTaskState struct {
	// rootPath is the root path that we're watching
	rootPath string

	// broadcaster is a pointer to the broadcaster.
	broadcaster *ipc.Broadcaster

	// broadcastChannel is the broadcast channel for this task.
	broadcastChannel chan ipc.Message

	// db is the sqlite database handle
	db *sql.DB
}

func NewProcTaskState(rootPath string, broadcaster *ipc.Broadcaster, broadcastChannel chan ipc.Message) *ProcTaskState {
	db, err := sql.Open("sqlite3", FSD_DB_FILENAME)
	if err != nil {
		zap.L().Fatal("failed to open sqlite database", zap.Error(err))
	}

	_, err = db.Exec(PROC_CREATE)
	if err != nil {
		zap.L().Fatal("failed to create proc table", zap.Error(err))
	}

	return &ProcTaskState{
		rootPath:         rootPath,
		broadcaster:      broadcaster,
		broadcastChannel: broadcastChannel,
		db:               db,
	}
}

func (p *ProcTaskState) RootPath() string {
	return p.rootPath
}

func (p *ProcTaskState) Broadcaster() *ipc.Broadcaster {
	return p.broadcaster
}

func (p *ProcTaskState) BroadcastChannel() chan ipc.Message {
	return p.broadcastChannel
}

// ProcTask takes tasks from the proc table and executes them as shell commands in a
// separate go task. It then updates the proc_results table with the results of the
// command. The API will give an associated ID to the command so that the client can
// poll for the results.
type ProcTask struct {
	state *ProcTaskState
}

// SendMessage implements Task.
func (p *ProcTask) SendMessage(msg ipc.Message) error {
	return nil
}

func ProcTaskName() string {
	return "ProcTask"
}

func NewProcTask(state *ProcTaskState) *ProcTask {
	return &ProcTask{
		state: state,
	}
}

func (p *ProcTask) StartEventLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			zap.L().Info("got shutdown signal, exiting", zap.String("task name", ProcTaskName()))
			return
		case <-time.After(time.Second * 1):
			if err := p.doTask(ctx); err != nil {
				zap.L().Error("failed to execute shell commands from the proc table", zap.Error(err))
			}
		}
	}
}

func (p *ProcTask) HandleMessage(ctx context.Context, msg ipc.Message) error {
	ms, err := msg.String()
	if err != nil {
		zap.L().Error("received invalid message", zap.Error(err))
		return err
	}
	zap.L().Debug("got message", zap.String("task name", ProcTaskName()), zap.String("msg", ms))

	return nil
}

// doTask takes a command from the proc table and executes it as a goroutine, storing std
// out and std err to the proc_results table as raw text.
func (p *ProcTask) doTask(ctx context.Context) error {
	query := `
		SELECT id, command, args, created_at FROM proc WHERE is_executed = 0
	`

	rows, err := p.state.db.Query(query)
	if err != nil {
		return err
	}

	for rows.Next() {
		var id int
		var command string
		var args string
		var createdAt time.Time
		err = rows.Scan(&id, &command, &args, &createdAt)
		if err != nil {
			return err
		}

		go func() {
			stdout, stderr, err := p.executeCommand(ctx, command, strings.Split(args, " "))
			if err != nil {
				zap.L().Error("failed to execute command", zap.Error(err))
				return
			}

			stmt, err := p.state.db.Prepare(`
				INSERT INTO proc_results (id, stdout, stderr, created_at) VALUES (?, ?, ?, ?)
			`)

			if err != nil {
				zap.L().Error("failed to prepare insert statement", zap.Error(err))
				return
			}

			_, err = stmt.Exec(id, stdout, stderr, createdAt)
			if err != nil {
				zap.L().Error("failed to insert into proc_results", zap.Error(err))
				return
			}

			_, err = p.state.db.Exec(`
				UPDATE proc SET is_executed = 1 WHERE id = ?
			`, id)

			if err != nil {
				zap.L().Error("failed to update proc table", zap.Error(err))
				return
			}
		}()
	}

	return nil
}

func (p *ProcTask) executeCommand(ctx context.Context, command string, args []string) (string, string, error) {
	zap.L().Debug("executing command", zap.String("command", command), zap.Any("args", args))

}
