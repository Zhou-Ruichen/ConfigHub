package render

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// Lock placement: render owns the hub-side render lock because it is only used
// to serialize local render operations before any bundle write begins.
var (
	ErrLockHeld  = errors.New("render lock is held")
	ErrStaleLock = errors.New("render lock is stale")
)

// AcquireRenderLock creates state/render.lock as a PID file. Existing live or
// stale locks are surfaced to callers; stale locks are never auto-cleared.
func AcquireRenderLock(rootStateDir string) (func(), error) {
	if err := os.MkdirAll(rootStateDir, 0o700); err != nil {
		return nil, fmt.Errorf("create state directory: %w", err)
	}
	if err := os.Chmod(rootStateDir, 0o700); err != nil {
		return nil, fmt.Errorf("chmod state directory: %w", err)
	}

	lockPath := rootStateDir + string(os.PathSeparator) + "render.lock"
	pid := os.Getpid()
	file, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err == nil {
		if _, writeErr := fmt.Fprintf(file, "%d\n", pid); writeErr != nil {
			_ = file.Close()
			_ = os.Remove(lockPath)
			return nil, fmt.Errorf("write render lock: %w", writeErr)
		}
		if closeErr := file.Close(); closeErr != nil {
			_ = os.Remove(lockPath)
			return nil, fmt.Errorf("close render lock: %w", closeErr)
		}
		return func() { _ = os.Remove(lockPath) }, nil
	}
	if !os.IsExist(err) {
		return nil, fmt.Errorf("create render lock: %w", err)
	}

	data, readErr := os.ReadFile(lockPath)
	if readErr != nil {
		return nil, fmt.Errorf("read existing render lock: %w", readErr)
	}
	lockedPID, parseErr := strconv.Atoi(strings.TrimSpace(string(data)))
	if parseErr != nil || lockedPID <= 0 {
		return nil, fmt.Errorf("%w: invalid PID in %s; run confighub lock release --force after confirming no render is active", ErrStaleLock, lockPath)
	}
	if processAlive(lockedPID) {
		return nil, fmt.Errorf("%w: PID %d is active", ErrLockHeld, lockedPID)
	}
	return nil, fmt.Errorf("%w: PID %d is not active; run confighub lock release --force after confirming no render is active", ErrStaleLock, lockedPID)
}

func processAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	return errors.Is(err, syscall.EPERM)
}
