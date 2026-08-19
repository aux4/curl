package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	lockPollInterval = 25 * time.Millisecond
	lockTimeout      = 15 * time.Second
	// A lock older than this belonged to a process that died before releasing
	// it: a token fetch that has taken this long has failed, not stalled.
	lockStaleAfter = 60 * time.Second
)

// withTokenLock serializes the read-modify-write cycle on a token file.
//
// Without it, `auth-stream --concurrency 10` (or simply two aux4 processes)
// can each notice the same expired token, fetch concurrently, and race on the
// write. Last writer wins, and with providers that invalidate the previous
// token when issuing a new one, the winner may not be the token the other
// in-flight requests are already carrying.
//
// O_CREATE|O_EXCL is the portable primitive: flock is not available across
// every platform aux4 cross-compiles for, exclusive create is.
func withTokenLock(path string, fn func() error) error {
	lockPath := path + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), 0700); err != nil {
		return err
	}

	deadline := time.Now().Add(lockTimeout)
	acquired := false

	for {
		file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err == nil {
			fmt.Fprintf(file, "%d", os.Getpid())
			file.Close()
			acquired = true
			break
		}
		if !os.IsExist(err) {
			return err
		}

		if info, statErr := os.Stat(lockPath); statErr == nil && time.Since(info.ModTime()) > lockStaleAfter {
			os.Remove(lockPath)
			continue
		}

		if time.Now().After(deadline) {
			// Continuing without the lock beats failing the user's request: the
			// worst case is the duplicate fetch the lock existed to prevent.
			break
		}
		time.Sleep(lockPollInterval)
	}

	if acquired {
		defer os.Remove(lockPath)
	}
	return fn()
}
