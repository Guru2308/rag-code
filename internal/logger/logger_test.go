package logger

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
)

func TestInit(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"json debug", Config{Level: LevelDebug, Format: "json"}, false},
		{"text info", Config{Level: LevelInfo, Format: "text"}, false},
		{"warn default", Config{Level: LevelWarn, Format: ""}, false},
		{"error invalid", Config{Level: LevelError, Format: "invalid"}, false}, // falls back to text
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Init(tt.cfg); (err != nil) != tt.wantErr {
				t.Errorf("Init() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGlobalLoggerWrappers(t *testing.T) {
	// Initialize with a buffer to prevent stdout noise, though real impl writes to os.Stdout directly
	// Since we can't easily swap out os.Stdout in parallel tests safely without race conditions or affecting other tests,
	// we will just ensure they don't panic. To verify content, we can manually swap defaultLogger

	Init(Config{Level: LevelDebug, Format: "text"})

	// Create a buffer and custom handler to verify output
	var buf bytes.Buffer
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	testLogger := slog.New(h)

	// Swap the library instance
	defaultLogger = testLogger

	// Test wrappers
	Debug("debug msg", "key", "val")
	Info("info msg", "key", "val")
	Warn("warn msg", "key", "val")
	Error("error msg", "key", "val")

	ctx := context.Background()
	DebugContext(ctx, "debug ctx", "key", "val")
	InfoContext(ctx, "info ctx", "key", "val")
	WarnContext(ctx, "warn ctx", "key", "val")
	ErrorContext(ctx, "error ctx", "key", "val")

	output := buf.String()
	if output == "" {
		t.Error("Expected output, got empty string")
	}
}

func TestWith(t *testing.T) {
	Init(Config{Level: LevelInfo})
	l := With("module", "test")
	if l == nil {
		t.Error("With() returned nil")
	}
}
