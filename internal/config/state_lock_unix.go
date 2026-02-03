//go:build unix

package config

import (
	"os"
	"syscall"
)

// acquireFileLock acquires an exclusive lock on the given file.
// Uses flock on Unix systems for proper file locking.
func acquireFileLock(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}

// releaseFileLock releases the lock on the given file.
func releaseFileLock(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
