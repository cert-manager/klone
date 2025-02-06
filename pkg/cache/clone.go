package cache

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/cert-manager/klone/pkg/mod"
)

func calculateCacheKey(src mod.KloneSource) string {
	return fmt.Sprintf("cache-%x", sha256.Sum256([]byte(fmt.Sprintf("%s-%s-%s", src.RepoURL, src.RepoHash, src.RepoPath))))[:30]
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

func CloneWithCache(
	destPath string,
	src mod.KloneSource,
	getFn func(targetPath string, src mod.KloneSource) (string, error),
) error {
	cacheDir, err := getCacheDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}

	cachePath := filepath.Join(cacheDir, calculateCacheKey(src))

	if _, err := os.Stat(cachePath); err != nil && !os.IsNotExist(err) {
		return err
	} else if err != nil {
		tempDir, err := os.MkdirTemp(cacheDir, "temp-*")
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

	if err := os.MkdirAll(destPath, 0755); err != nil {
		return err
	}

	if err := runRsyncCmd(cachePath, os.Stdout, os.Stderr, "-aEq", "--delete", ".", destPath); err != nil {
		return err
	}

	return nil
}

func runRsyncCmd(root string, stdout io.Writer, stderr io.Writer, args ...string) error {
	cmd := exec.Command("rsync", args...)

	cmd.Dir = root
	cmd.Env = append(os.Environ(), cmd.Env...)

	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("rsync command failed: %v", err)
	}

	return nil
}
