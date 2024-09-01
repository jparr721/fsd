package ipc

import (
	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

type FsdOp uint32

const (
	// A new file was created.
	Create FsdOp = iota

	// A file was written to, or is currently being written to.
	Write

	// A file path was removed.
	Remove

	// A path was renamed.
	Rename

	// File attributed changed.
	Chmod

	// An invalid operation was given
	Invalid
)

func (o FsdOp) String() string {
	switch o {
	case Create:
		return "Create"
	case Chmod:
		return "Chmod"
	case Remove:
		return "Remove"
	case Rename:
		return "Rename"
	case Write:
		return "Write"
	default:
		return "InvalidOperation"
	}
}

func NewFsdOpFromINotifyOp(iNotifyOp fsnotify.Op) FsdOp {
	switch iNotifyOp {
	case fsnotify.Chmod:
		return Chmod
	case fsnotify.Create:
		return Create
	case fsnotify.Remove:
		return Remove
	case fsnotify.Rename:
		return Rename
	case fsnotify.Write:
		return Write
	default:
		zap.L().Error("Unrecognized inotify operation found", zap.Any("inotify operation", iNotifyOp))
		return Invalid
	}
}

type Message interface {
	// String converts a message into a JSON string
	String() (string, error)
	EventName() string
	EventOperation() FsdOp
}
