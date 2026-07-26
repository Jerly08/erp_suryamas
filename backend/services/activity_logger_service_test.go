package services

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewActivityLoggerDisablesFileLoggingWhenLogDirectoryIsUnavailable(t *testing.T) {
	t.Parallel()

	parentFile := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(parentFile, []byte("file"), 0600); err != nil {
		t.Fatalf("create parent file: %v", err)
	}

	logger := NewActivityLoggerService(nil, filepath.Join(parentFile, "logs"))
	if logger.enableFile {
		t.Fatal("expected file logging to be disabled when its directory cannot be created")
	}
	if !logger.enableDB {
		t.Fatal("database logging must remain enabled when file logging is unavailable")
	}
}

func TestNewActivityLoggerEnablesFileLoggingForWritableDirectory(t *testing.T) {
	t.Parallel()

	logDir := filepath.Join(t.TempDir(), "logs")
	logger := NewActivityLoggerService(nil, logDir)
	if !logger.enableFile {
		t.Fatal("expected file logging to be enabled for a writable directory")
	}

	entries, err := os.ReadDir(logDir)
	if err != nil {
		t.Fatalf("read log directory: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected write probe to be removed, found %d entries", len(entries))
	}
}
