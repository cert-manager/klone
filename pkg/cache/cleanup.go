package cache

import (
	"os"
	"path/filepath"
	"time"
)

func CleanupOldCacheItems() error {
	cacheDir, err := getCacheDir()
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if time.Since(info.ModTime()) > 7*24*time.Hour {
			if err := os.RemoveAll(filepath.Join(cacheDir, entry.Name())); err != nil {
				return err
			}
		}
	}

	return nil
}
