package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const vaultWatchDebounce = 200 * time.Millisecond

// vaultWatcher observes the vault directory because writes replace the vault
// file via rename, which does not emit events on the old inode.
type vaultWatcher struct {
	watcher *fsnotify.Watcher
	path    string
	stop    chan struct{}
	once    sync.Once
}

func startVaultWatcher(path string, onChange func()) (*vaultWatcher, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(absolute)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := watcher.Add(dir); err != nil {
		_ = watcher.Close()
		return nil, err
	}
	instance := &vaultWatcher{
		watcher: watcher,
		path:    filepath.Clean(absolute),
		stop:    make(chan struct{}),
	}
	go instance.run(onChange)
	return instance, nil
}

// Close is deliberately non-blocking: a callback may already be waiting for
// the application main thread, and shutdown runs on that same thread.
func (w *vaultWatcher) Close() {
	w.once.Do(func() {
		close(w.stop)
		_ = w.watcher.Close()
	})
}

func (w *vaultWatcher) run(onChange func()) {
	var timer *time.Timer
	var timerC <-chan time.Time
	for {
		select {
		case <-w.stop:
			if timer != nil {
				timer.Stop()
			}
			return
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			if !w.matches(event.Name) {
				continue
			}
			if timer == nil {
				timer = time.NewTimer(vaultWatchDebounce)
			} else {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(vaultWatchDebounce)
			}
			timerC = timer.C
		case <-timerC:
			timer = nil
			timerC = nil
			if onChange != nil {
				onChange()
			}
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("2fa: наблюдение за хранилищем: %v", err)
		}
	}
}

func (w *vaultWatcher) matches(name string) bool {
	if strings.HasSuffix(name, ".lock") {
		return false
	}
	return filepath.Clean(name) == w.path
}
