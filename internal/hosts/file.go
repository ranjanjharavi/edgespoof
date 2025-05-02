package hosts

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func (m *Manager) lockFile() (*os.File, error) {
	lock, err := os.OpenFile(m.Path+".edgespoof.lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open hosts lock file: %w", err)
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		lock.Close()
		return nil, fmt.Errorf("failed to lock hosts file: %w", err)
	}

	// Keep the file. Removing it can leave waiting writers locking different inodes
	// Closing the descriptor releases the lock
	return lock, nil
}

func (m *Manager) writeAtomic(finalData string) error {
	tmpFile, err := os.CreateTemp(filepath.Dir(m.Path), ".hosts.edgespoof.*")
	if err != nil {
		return fmt.Errorf("failed to create temp hosts file (verify sudo?): %w", err)
	}

	tmpName := tmpFile.Name()
	defer os.Remove(tmpName)

	if _, err := tmpFile.WriteString(finalData); err != nil {
		tmpFile.Close()
		return err
	}
	if err := tmpFile.Chmod(0644); err != nil {
		tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpName, m.Path); err != nil {
		// Bind-mounted hosts files may reject replacement.
		if errors.Is(err, syscall.EXDEV) || errors.Is(err, syscall.EBUSY) {
			return os.WriteFile(m.Path, []byte(finalData), 0644)
		}
		return err
	}
	return nil
}
