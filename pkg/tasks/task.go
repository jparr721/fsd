package tasks

import (
	"context"
	"fsd/pkg/ipc"
)

type TaskState interface {
	// RootPath returns the root path of the project.
	RootPath() string

	// Broadcaster returns a pointer to the broadcaster.
	Broadcaster() *ipc.Broadcaster

	// BroadcastChannel returns the broadcast channel for this task.
	BroadcastChannel() chan ipc.Message
}

type Task interface {
	// StartEventLoop starts the infinite loop awaiting events or shutdown
	StartEventLoop(ctx context.Context)

	// HandleMessage handles a network message
	HandleMessage(ctx context.Context, msg ipc.Message) error

	// SendMessage sends a message over the network
	SendMessage(msg ipc.Message) error
}
