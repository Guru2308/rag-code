package indexing

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Guru2308/rag-code/internal/logger"
)

func init() {
	logger.Init(logger.Config{Level: logger.LevelDebug})
}

func TestNewWatcher(t *testing.T) {
	handler := func(ctx context.Context, path string, event FileEvent) error {
		return nil
	}

	watcher, err := NewWatcher(handler)
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}
	defer watcher.Stop()

	if watcher == nil {
		t.Error("NewWatcher() returned nil watcher")
	}
}

func TestNewWatcher_NilHandler(t *testing.T) {
	_, err := NewWatcher(nil)
	if err == nil {
		t.Error("NewWatcher() expected error for nil handler")
	}
}

func TestWatcher_AddPath(t *testing.T) {
	handler := func(ctx context.Context, path string, event FileEvent) error {
		return nil
	}

	watcher, err := NewWatcher(handler)
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}
	defer watcher.Stop()

	// Create a temp directory to watch
	tmpDir, err := os.MkdirTemp("", "watcher_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	err = watcher.AddPath(tmpDir)
	if err != nil {
		t.Errorf("AddPath() error = %v", err)
	}
}

func TestWatcher_AddPath_InvalidPath(t *testing.T) {
	handler := func(ctx context.Context, path string, event FileEvent) error {
		return nil
	}

	watcher, err := NewWatcher(handler)
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}
	defer watcher.Stop()

	err = watcher.AddPath("")
	if err == nil {
		t.Error("AddPath() expected error for empty path")
	}
}

func TestWatcher_Start_ContextCancellation(t *testing.T) {
	handler := func(ctx context.Context, path string, event FileEvent) error {
		return nil
	}

	watcher, err := NewWatcher(handler)
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	
	// Start watcher in goroutine
	done := make(chan error, 1)
	go func() {
		done <- watcher.Start(ctx)
	}()

	// Cancel immediately
	cancel()

	// Wait for completion
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Start() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Start() did not complete after context cancellation")
	}
}

func TestWatcher_Stop(t *testing.T) {
	handler := func(ctx context.Context, path string, event FileEvent) error {
		return nil
	}

	watcher, err := NewWatcher(handler)
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}

	err = watcher.Stop()
	if err != nil {
		t.Errorf("Stop() error = %v", err)
	}
}

func TestFileEvent_Constants(t *testing.T) {
	if FileEventCreate != "create" {
		t.Errorf("FileEventCreate = %v, want 'create'", FileEventCreate)
	}
	if FileEventModify != "modify" {
		t.Errorf("FileEventModify = %v, want 'modify'", FileEventModify)
	}
	if FileEventDelete != "delete" {
		t.Errorf("FileEventDelete = %v, want 'delete'", FileEventDelete)
	}
}

func TestWatcher_HandleEvent_Create(t *testing.T) {
	eventReceived := make(chan FileEvent, 1)
	handler := func(ctx context.Context, path string, event FileEvent) error {
		eventReceived <- event
		return nil
	}

	watcher, err := NewWatcher(handler)
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}
	defer watcher.Stop()

	tmpDir, err := os.MkdirTemp("", "watcher_event_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	err = watcher.AddPath(tmpDir)
	if err != nil {
		t.Fatalf("AddPath() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go watcher.Start(ctx)

	// Give the watcher time to start
	time.Sleep(100 * time.Millisecond)

	// Create a file
	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Wait for event
	select {
	case event := <-eventReceived:
		if event != FileEventCreate {
			t.Errorf("Expected FileEventCreate, got %v", event)
		}
	case <-time.After(2 * time.Second):
		t.Error("Timed out waiting for file creation event")
	}
}
