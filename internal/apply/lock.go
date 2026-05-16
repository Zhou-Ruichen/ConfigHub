package apply

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

var (
	ErrLockHeld  = errors.New("apply lock is held")
	ErrStaleLock = errors.New("apply lock is stale")
)

// AcquireApplyLock creates state/apply.lock as a PID file. Slice 3 keeps one
// process-wide apply lock and includes the profile id in the lock content for a
// future per-profile relaxation.
func AcquireApplyLock(rootStateDir, profileID string) (func(), error) {
	if err := os.MkdirAll(rootStateDir, 0o700); err != nil {
		return nil, fmt.Errorf("create state directory: %w", err)
	}
	if err := os.Chmod(rootStateDir, 0o700); err != nil {
		return nil, fmt.Errorf("chmod state directory: %w", err)
	}

	lockPath := filepath.Join(rootStateDir, "apply.lock")
	pid := os.Getpid()
	file, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err == nil {
		if _, writeErr := fmt.Fprintf(file, "%d\n%s\n", pid, profileID); writeErr != nil {
			_ = file.Close()
			_ = os.Remove(lockPath)
			return nil, fmt.Errorf("write apply lock: %w", writeErr)
		}
		if closeErr := file.Close(); closeErr != nil {
			_ = os.Remove(lockPath)
			return nil, fmt.Errorf("close apply lock: %w", closeErr)
		}
		return func() { _ = os.Remove(lockPath) }, nil
	}
	if !os.IsExist(err) {
		return nil, fmt.Errorf("create apply lock: %w", err)
	}

	data, readErr := os.ReadFile(lockPath)
	if readErr != nil {
		return nil, fmt.Errorf("read existing apply lock: %w", readErr)
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return nil, fmt.Errorf("%w: invalid PID in %s; run confighub lock release --force after confirming no apply is active", ErrStaleLock, lockPath)
	}
	lockedPID, parseErr := strconv.Atoi(fields[0])
	if parseErr != nil || lockedPID <= 0 {
		return nil, fmt.Errorf("%w: invalid PID in %s; run confighub lock release --force after confirming no apply is active", ErrStaleLock, lockPath)
	}
	if processAlive(lockedPID) {
		return nil, fmt.Errorf("%w: PID %d is active", ErrLockHeld, lockedPID)
	}
	return nil, fmt.Errorf("%w: PID %d is not active; run confighub lock release --force after confirming no apply is active", ErrStaleLock, lockedPID)
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
