//go:build !unix && !windows

package core

import "os"

// Platforms without flock/LockFileEx fall back to process-local behaviour.
func lockFile(*os.File, bool) error { return nil }

func unlockFile(*os.File) error { return nil }
