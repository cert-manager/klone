package cache

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cert-manager/klone/pkg/mod"
	cp "github.com/otiai10/copy"
)

func calculateCacheKey(src mod.KloneSource) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%s-%s-%s", src.RepoURL, src.RepoHash, src.RepoPath))))[:30]
}

func getCacheDir() (string, error) {
	if cacheDir := os.Getenv("KLONE_CACHE_DIR"); cacheDir != "" {
		return filepath.Abs(filepath.Clean(cacheDir))
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Abs(filepath.Clean(filepath.Join(home, ".cache", "klone")))
}

func getTempDir() (string, error) {
	if tempDir := os.Getenv("KLONE_TEMP_DIR"); tempDir != "" {
		return filepath.Abs(filepath.Clean(tempDir))
	}

	return os.TempDir(), nil
}

func CloneWithCache(
	destPath string,
	src mod.KloneSource,
	getFn func(targetPath string, src mod.KloneSource) (string, error),
) error {
	cacheDir, err := getCacheDir()
	if err != nil {
		return err
	}

	cachePath := filepath.Join(cacheDir, calculateCacheKey(src))

	if _, err := os.Stat(cachePath); err != nil && !os.IsNotExist(err) {
		return err
	} else if err != nil {
		tmpParentDir, err := getTempDir()
		if err != nil {
			return err
		}

		tempDir, err := os.MkdirTemp(tmpParentDir, "klone-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tempDir)

		outPath, err := getFn(tempDir, src)
		if err != nil {
			return err
		}

		// remove .git folder from outPath (if it exists)
		if err := os.RemoveAll(filepath.Join(outPath, ".git")); err != nil {
			return err
		}

		if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
			return err
		}

		if err := os.Rename(outPath, cachePath); err != nil {
			return err
		}
	}

	currentTime := time.Now()
	if err := os.Chtimes(cachePath, currentTime, currentTime); err != nil {
		return err
	}

	if err := os.RemoveAll(destPath); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	if err := cp.Copy(cachePath, destPath); err != nil {
		return err
	}

	return nil
}
