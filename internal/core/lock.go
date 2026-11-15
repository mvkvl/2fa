package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// WithVaultLock runs fn while holding an advisory lock for the vault at path.
// Writers take an exclusive lock, readers a shared one. The lock is stored in a
// sibling "<path>.lock" file that is intentionally never removed: deleting it
// would allow processes holding different inodes to bypass each other.
func WithVaultLock(path string, exclusive bool, fn func() error) error {
	if path == "" {
		return errors.New("путь к хранилищу не указан")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("не удалось создать каталог блокировки: %w", err)
	}
	lockPath := path + ".lock"
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("не удалось открыть файл блокировки: %w", err)
	}
	defer file.Close()
	if err := lockFile(file, exclusive); err != nil {
		return fmt.Errorf("не удалось заблокировать хранилище: %w", err)
	}
	defer unlockFile(file)
	if fn == nil {
		return nil
	}
	return fn()
}
