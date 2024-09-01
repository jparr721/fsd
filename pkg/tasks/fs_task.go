package tasks

import (
	"context"
	"encoding/json"
	"fsd/pkg/ipc"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

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
}

func NewFsTaskState(rootPath string, broadcaster *ipc.Broadcaster, broadcastChannel chan ipc.Message) *FsTaskState {
	return &FsTaskState{
		rootPath:         rootPath,
		broadcaster:      broadcaster,
		broadcastChannel: broadcastChannel,
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

func (fs *FsTask) StartEventLoop(ctx context.Context) {
	for {
		select {
		case event := <-fs.state.BroadcastChannel():
			fs.HandlMessage(event)
		case <-ctx.Done():
			zap.L().Info("got shutdown signal, exiting", zap.String("task name", FsTaskName()))
			return
		}
	}
}

// HandleMessage handles a network message
func (fs *FsTask) HandlMessage(msg ipc.Message) error {
	ms, err := msg.String()
	if err != nil {
		zap.L().Error("Received invalid message", zap.Error(err))
		return err
	}
	zap.L().Info("got message", zap.String("task name", FsTaskName()), zap.String("msg", ms))
	return nil
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
