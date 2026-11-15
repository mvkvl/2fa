package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWithVaultLockSerializesWriters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "accounts.json")

	acquired := make(chan struct{})
	release := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- WithVaultLock(path, true, func() error {
			close(acquired)
			<-release
			return nil
		})
	}()
	<-acquired

	secondEntered := make(chan struct{})
	secondDone := make(chan error, 1)
	go func() {
		secondDone <- WithVaultLock(path, true, func() error {
			close(secondEntered)
			return nil
		})
	}()

	select {
	case <-secondEntered:
		t.Fatal("second writer acquired the lock while it was held")
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first lock: %v", err)
	}
	select {
	case <-secondEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("second writer did not acquire the lock after release")
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("second lock: %v", err)
	}

	lockPath := path + ".lock"
	info, err := os.Stat(lockPath)
	if err != nil {
		t.Fatalf("lock file missing: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("lock file mode = %o, want 0600", perm)
	}
}

func TestWithVaultLockAllowsConcurrentReaders(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accounts.json")

	acquired := make(chan struct{})
	release := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- WithVaultLock(path, false, func() error {
			close(acquired)
			<-release
			return nil
		})
	}()
	<-acquired

	secondEntered := make(chan struct{})
	secondDone := make(chan error, 1)
	go func() {
		secondDone <- WithVaultLock(path, false, func() error {
			close(secondEntered)
			return nil
		})
	}()

	select {
	case <-secondEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("second reader did not acquire a shared lock")
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("second lock: %v", err)
	}

	close(release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first lock: %v", err)
	}
}
