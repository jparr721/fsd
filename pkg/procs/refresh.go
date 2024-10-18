package procs

import (
	"context"
	"database/sql"
	"fsd/internal/config"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

type RefreshProc struct {
	ID int

	// Cmd is the command that the proc uses to execute the refresh command
	Cmd string

	// Args are the arguments that the proc uses to execute the refresh command
	Args []string

	// db is the database that the proc uses to store the output of the refresh command
	db *sql.DB
}

func RefreshProcName() string {
	return "refresh"
}

func NewRefreshProc(ctx context.Context, url string, playlistEnd int) (*RefreshProc, error) {
	db, err := sql.Open("sqlite3", config.GetDBPath())
	if err != nil {
		zap.L().Fatal("failed to open sqlite database", zap.Error(err))
		return nil, err
	}

	cmd := "yt-dlp"
	args := []string{url, "--print", "%(upload_date>%Y-%m-%d)s %(title)s %(id)s", url, "--playlist-end", strconv.Itoa(playlistEnd)}

	// Store the proc in the database
	result, err := db.Exec(`
		INSERT INTO proc (command, args, is_executed, created_at) VALUES (?, ?, ?, ?)
	`, cmd, strings.Join(args, " "), 0, time.Now())
	if err != nil {
		zap.L().Error("failed to insert into procs", zap.String("proc", YtProcName()), zap.Error(err))
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		zap.L().Error("failed to get last insert id", zap.String("proc", YtProcName()), zap.Error(err))
		return nil, err
	}

	return &RefreshProc{
		ID:   int(id),
		Cmd:  cmd,
		Args: args,
		db:   db,
	}, nil
}

func (p *RefreshProc) GetID() int {
	return p.ID
}

func (p *RefreshProc) GetCmd() string {
	return p.Cmd
}

func (p *RefreshProc) GetArgs() []string {
	return p.Args
}
