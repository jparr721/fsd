package procs

import (
	"context"
	"io"
	"os/exec"

	"go.uber.org/zap"
)

type Proc interface {
	// GetID returns the id of the proc
	GetID() int

	// GetCmd returns the command of the proc
	GetCmd() string

	// GetArgs returns the arguments of the proc
	GetArgs() []string
}

func Execute(ctx context.Context, command string, args []string) (string, string, error) {
	zap.L().Debug("executing command", zap.String("command", command), zap.Any("args", args))
	cmd := exec.CommandContext(ctx, command, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		zap.L().Error("error creating stdout pipe", zap.Error(err))
		return "", "", err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		zap.L().Error("error creating stderr pipe", zap.Error(err))
		return "", "", err
	}

	if err := cmd.Start(); err != nil {
		zap.L().Error("error starting command", zap.Error(err))
		return "", "", err
	}

	output, err := io.ReadAll(stdout)
	if err != nil {
		zap.L().Error("error reading stdout", zap.Error(err))
		return "", "", err
	}

	stderrOutput, err := io.ReadAll(stderr)
	if err != nil {
		zap.L().Error("error reading stderr", zap.Error(err))
		return "", "", err
	}

	if err := cmd.Wait(); err != nil {
		zap.L().Error("error waiting for command", zap.Error(err))
		return "", "", err
	}

	return string(output), string(stderrOutput), nil
}
