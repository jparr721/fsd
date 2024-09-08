package procs

import (
	"context"
	"database/sql"
	"fmt"
	"fsd/internal/config"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"go.uber.org/zap"
)

var DEFAULT_FLAGS = map[string][]string{
	"playlist-end":        {"30"},
	"sleep-interval":      {"5"},
	"merge-output-format": {"mkv"},
}

type YtProc struct {
	Id int

	// Cmd is the command that the proc uses to execute the yt-dlp command
	Cmd string

	// Args are the arguments that the proc uses to execute the yt-dlp command
	Args []string

	// db is the database that the proc uses to store the output of the yt-dlp command
	db *sql.DB
}

func YtProcName() string {
	return "yt-dlp"
}

func NewYtProc(ctx context.Context, overrides map[string][]string) (*YtProc, error) {
	db, err := sql.Open("sqlite3", config.GetDBPath())
	if err != nil {
		zap.L().Error("failed to open sqlite database", zap.Error(err))
		return nil, err
	}

	// Start with the default flags
	flags := make(map[string][]string)
	for k, v := range DEFAULT_FLAGS {
		flags[k] = v
	}

	// Apply overrides
	for k, v := range overrides {
		flags[k] = v
	}

	// The only required flag is the url
	url, ok := flags["url"]
	if !ok {
		return nil, fmt.Errorf("url is required")
	}

	cmd := "yt-dlp"
	args := []string{url[0]}

	// Add all flags to the args slice
	for k, v := range flags {
		if k != "url" && k != "channel-name" { // Skip the URL as we've already added it
			args = append(args, fmt.Sprintf("--%s", k))
			args = append(args, v...)
		}
	}

	// Add the output path
	channelName := flags["channel-name"][0]
	outputPath := makePath(channelName)
	args = append(args, "-o", fmt.Sprintf("%s/%%(title)s", outputPath))

	// Store the proc in the database
	result, err := db.Exec(`
		INSERT INTO proc (command, args, is_executed, created_at) VALUES (?, ?, ?, ?)
	`, cmd, strings.Join(args, " "), 0, time.Now())
	if err != nil {
		zap.L().Error("failed to insert into procs", zap.Error(err))
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		zap.L().Error("failed to get last insert id", zap.Error(err))
		return nil, err
	}

	return &YtProc{
		Id:   int(id),
		Cmd:  cmd,
		Args: args,
		db:   db,
	}, nil
}

func (p *YtProc) GetId() int {
	return p.Id
}

func (p *YtProc) GetCmd() string {
	return p.Cmd
}

func (p *YtProc) GetArgs() []string {
	return p.Args
}

func makePath(channelName string) string {
	return filepath.Join(config.GetConfig().WatchDir, channelName)
}
