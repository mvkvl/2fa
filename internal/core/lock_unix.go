//go:build unix

package core

import (
	"os"
	"syscall"
)

func lockFile(file *os.File, exclusive bool) error {
	operation := syscall.LOCK_SH
	if exclusive {
		operation = syscall.LOCK_EX
	}
	for {
		err := syscall.Flock(int(file.Fd()), operation)
		if err != syscall.EINTR {
			return err
		}
	}
}

func unlockFile(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}
