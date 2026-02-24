package indexing

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Guru2308/rag-code/internal/errors"
	"github.com/Guru2308/rag-code/internal/logger"
	"github.com/Guru2308/rag-code/internal/validator"
	"github.com/fsnotify/fsnotify"
)

// Watcher monitors file system changes and triggers indexing
type Watcher struct {
	watcher          *fsnotify.Watcher
	paths            []string
	handler          ChangeHandler
	mu               sync.RWMutex
	debounceDuration time.Duration
	pending          map[string]*time.Timer
	pendingMu        sync.Mutex
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

// NewWatcher creates a new file system watcher.
// debounceDuration controls how long to wait after the last event before
// calling the handler. Use 0 to disable debouncing.
func NewWatcher(handler ChangeHandler, debounceDuration time.Duration) (*Watcher, error) {
	if handler == nil {
		return nil, errors.ValidationError("change handler cannot be nil")
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeInternal, "failed to create file watcher")
	}

	if debounceDuration <= 0 {
		debounceDuration = 500 * time.Millisecond
	}

	return &Watcher{
		watcher:          w,
		paths:            make([]string, 0),
		handler:          handler,
		debounceDuration: debounceDuration,
		pending:          make(map[string]*time.Timer),
	}, nil
}

// AddPath adds a directory to watch, recursively including all subdirectories.
// fsnotify does not support recursive watching natively, so we walk the tree.
func (w *Watcher) AddPath(path string) error {
	if err := validator.ValidateFilePath(path); err != nil {
		return err
	}

	absPath, _ := filepath.Abs(path)

	w.mu.Lock()
	defer w.mu.Unlock()

	var dirsAdded int
	err := filepath.Walk(absPath, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable paths
		}
		if !info.IsDir() {
			return nil
		}
		// Skip hidden dirs and common noise
		name := info.Name()
		if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
			return filepath.SkipDir
		}
		if err := w.watcher.Add(p); err != nil {
			logger.Warn("Failed to watch directory", "path", p, "error", err)
			return nil
		}
		dirsAdded++
		return nil
	})
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeInternal, "failed to walk watch path")
	}

	w.paths = append(w.paths, absPath)
	logger.Info("Added watch path (recursive)", "root", absPath, "dirs_watched", dirsAdded)

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

// handleEvent processes a file system event with debouncing.
// Rapid successive events for the same file are collapsed into one handler call.
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
		return // Ignore rename/chmod etc.
	}

	logger.Info("File event detected",
		"path", event.Name,
		"event", fileEvent,
	)

	path := event.Name

	w.pendingMu.Lock()
	// Cancel any existing timer for this path
	if t, ok := w.pending[path]; ok {
		t.Stop()
	}
	// Schedule a new timer
	w.pending[path] = time.AfterFunc(w.debounceDuration, func() {
		w.pendingMu.Lock()
		delete(w.pending, path)
		w.pendingMu.Unlock()

		if err := w.handler(ctx, path, fileEvent); err != nil {
			logger.Error("Failed to handle file event",
				"path", path,
				"event", fileEvent,
				"error", err,
			)
		}
	})
	w.pendingMu.Unlock()
}

// Stop stops the watcher and cancels all pending debounce timers.
func (w *Watcher) Stop() error {
	w.pendingMu.Lock()
	for _, t := range w.pending {
		t.Stop()
	}
	w.pending = make(map[string]*time.Timer)
	w.pendingMu.Unlock()
	return w.watcher.Close()
}
