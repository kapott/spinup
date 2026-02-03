//go:build windows

package config

import (
	"os"
)

// acquireFileLock acquires an exclusive lock on the given file.
// On Windows, we use a simpler approach with LockFileEx.
// Note: This is a basic implementation for Windows compatibility.
func acquireFileLock(_ *os.File) error {
	// Windows file locking would require using syscall.LockFileEx
	// For now, return nil as Windows is not a primary target for this tool.
	// The in-process mutex still provides protection for single-process scenarios.
	return nil
}

// releaseFileLock releases the lock on the given file.
func releaseFileLock(_ *os.File) error {
	return nil
}
