package indexing

import (
	"context"
	"path/filepath"
	"sync"

	"github.com/Guru2308/rag-code/internal/errors"
	"github.com/Guru2308/rag-code/internal/logger"
	"github.com/Guru2308/rag-code/internal/validator"
	"github.com/fsnotify/fsnotify"
)

// Watcher monitors file system changes and triggers indexing
type Watcher struct {
	watcher *fsnotify.Watcher
	paths   []string
	handler ChangeHandler
	mu      sync.RWMutex
}

// ChangeHandler is called when files change
type ChangeHandler func(ctx context.Context, path string, event FileEvent) error

// FileEvent represents a file system event
type FileEvent string

const (
	FileEventCreate FileEvent = "create"
	FileEventModify FileEvent = "modify"
	FileEventDelete FileEvent = "delete"
)

// NewWatcher creates a new file system watcher
func NewWatcher(handler ChangeHandler) (*Watcher, error) {
	if handler == nil {
		return nil, errors.ValidationError("change handler cannot be nil")
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeInternal, "failed to create file watcher")
	}

	return &Watcher{
		watcher: w,
		paths:   make([]string, 0),
		handler: handler,
	}, nil
}

// AddPath adds a directory to watch
func (w *Watcher) AddPath(path string) error {
	if err := validator.ValidateFilePath(path); err != nil {
		return err
	}

	absPath, _ := filepath.Abs(path)

	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.watcher.Add(absPath); err != nil {
		return errors.Wrap(err, errors.ErrorTypeInternal, "failed to add watch path")
	}

	w.paths = append(w.paths, absPath)
	logger.Info("Added watch path", "path", absPath)

	return nil
}

// Start begins watching for file changes
func (w *Watcher) Start(ctx context.Context) error {
	logger.Info("Starting file watcher", "paths", len(w.paths))

	for {
		select {
		case <-ctx.Done():
			logger.Info("File watcher stopped")
			return w.watcher.Close()

		case event, ok := <-w.watcher.Events:
			if !ok {
				return nil
			}
			w.handleEvent(ctx, event)

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return nil
			}
			logger.Error("File watcher error", "error", err)
		}
	}
}

// handleEvent processes a file system event
func (w *Watcher) handleEvent(ctx context.Context, event fsnotify.Event) {
	var fileEvent FileEvent

	switch {
	case event.Op&fsnotify.Create == fsnotify.Create:
		fileEvent = FileEventCreate
	case event.Op&fsnotify.Write == fsnotify.Write:
		fileEvent = FileEventModify
	case event.Op&fsnotify.Remove == fsnotify.Remove:
		fileEvent = FileEventDelete
	default:
		return // Ignore other events
	}

	logger.Debug("File event detected",
		"path", event.Name,
		"event", fileEvent,
	)

	if err := w.handler(ctx, event.Name, fileEvent); err != nil {
		logger.Error("Failed to handle file event",
			"path", event.Name,
			"event", fileEvent,
			"error", err,
		)
	}
}

// Stop stops the watcher
func (w *Watcher) Stop() error {
	return w.watcher.Close()
}

// TODO: Implement recursive directory watching
// TODO: Add file filtering (ignore .git, node_modules, etc.)
// TODO: Add debouncing for rapid file changes
