package render

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAcquireRenderLockContention(t *testing.T) {
	stateDir := t.TempDir()
	release, err := AcquireRenderLock(stateDir)
	if err != nil {
		t.Fatalf("AcquireRenderLock() error = %v", err)
	}
	defer release()

	secondRelease, err := AcquireRenderLock(stateDir)
	if err == nil {
		secondRelease()
		t.Fatalf("second AcquireRenderLock() error = nil, want lock held")
	}
	if !errors.Is(err, ErrLockHeld) {
		t.Fatalf("second AcquireRenderLock() error = %v, want ErrLockHeld", err)
	}
}

func TestAcquireRenderLockStalePID(t *testing.T) {
	stateDir := t.TempDir()
	lockPath := filepath.Join(stateDir, "render.lock")
	if err := os.WriteFile(lockPath, []byte("999999\n"), 0o600); err != nil {
		t.Fatalf("write stale lock: %v", err)
	}

	release, err := AcquireRenderLock(stateDir)
	if err == nil {
		release()
		t.Fatalf("AcquireRenderLock() error = nil, want stale lock")
	}
	if !errors.Is(err, ErrStaleLock) {
		t.Fatalf("AcquireRenderLock() error = %v, want ErrStaleLock", err)
	}
	data, readErr := os.ReadFile(lockPath)
	if readErr != nil {
		t.Fatalf("lock was removed, read error = %v", readErr)
	}
	if string(data) != "999999\n" {
		t.Fatalf("lock data = %q, want stale PID preserved", string(data))
	}
}
