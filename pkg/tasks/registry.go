package tasks

import (
	"context"
	"fsd/pkg/ipc"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

// TaskRegistry is a registry of all running tasks.
type TaskRegistry struct {
	// tasks maps a string identifier to a task name
	tasks map[string]Task
}

func NewTaskRegistry() *TaskRegistry {
	return &TaskRegistry{
		tasks: make(map[string]Task),
	}
}

func (t *TaskRegistry) Init(rootPath string, broadcaster *ipc.Broadcaster, watcher *fsnotify.Watcher, names ...string) {
	for _, name := range names {
		taskChan := broadcaster.Subscribe(name)
		switch name {
		case FsTaskName():
			taskState := NewFsTaskState(rootPath, broadcaster, taskChan)
			task := NewFsTask(taskState)
			t.tasks[FsTaskName()] = task
		case MetadataTaskName():
			taskState := NewMetadataTaskState(rootPath, broadcaster, taskChan, watcher)
			task := NewMetadataTask(taskState)
			t.tasks[MetadataTaskName()] = task
		}
	}
}

func (t *TaskRegistry) Run(ctx context.Context) {
	for name, task := range t.tasks {
		zap.L().Info("starting task", zap.String("task name", name))
		go task.StartEventLoop(ctx)
	}
}
