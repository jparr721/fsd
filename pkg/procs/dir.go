package procs

import (
	"context"
	"database/sql"
	"fsd/internal/config"
	"path/filepath"
	"time"

	"go.uber.org/zap"
)

type DirProc struct {
	ID int

	Cmd string

	// Args is the directory name and any additional flags
	Args []string

	db *sql.DB
}

func DirProcName() string {
	return "mkdir"
}

func NewDirProc(ctx context.Context, dirname string) (*DirProc, error) {
	db, err := sql.Open("sqlite3", config.GetDBPath())
	if err != nil {
		zap.L().Fatal("failed to open sqlite database", zap.Error(err))
		return nil, err
	}

	cmd := "mkdir"

	// Always make directories in the root path
	dirname = filepath.Join(config.GetConfig().WatchDir, dirname)

	// Store in the database
	result, err := db.Exec(`
		INSERT INTO proc (command, args, is_executed, created_at) VALUES (?, ?, ?, ?)
	`, cmd, dirname, 0, time.Now())
	if err != nil {
		zap.L().Error("failed to insert into procs", zap.String("proc", YtProcName()), zap.Error(err))
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		zap.L().Error("failed to get last insert id", zap.String("proc", YtProcName()), zap.Error(err))
		return nil, err
	}

	return &DirProc{
		ID:   int(id),
		Cmd:  cmd,
		Args: []string{dirname},
		db:   db,
	}, nil
}

func (p *DirProc) GetID() int {
	return p.ID
}

func (p *DirProc) GetCmd() string {
	return p.Cmd
}

func (p *DirProc) GetArgs() []string {
	return p.Args
}
