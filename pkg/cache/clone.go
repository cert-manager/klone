/*
Copyright 2023 The cert-manager Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cache

import (
	"context"
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
	return fmt.Sprintf("cache-%x", sha256.Sum256(fmt.Appendf(nil, "%s-%s-%s", src.RepoURL, src.RepoHash, src.RepoPath)))[:30]
}

func getCacheDir() (string, error) {
	// TODO: add centralized config management defining env vars + maybe a global config file for klone
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
	ctx context.Context,
	destPath string,
	src mod.KloneSource,
	getFn func(getCtx context.Context, targetPath string, src mod.KloneSource) (string, error),
) error {
	cacheDir, err := getCacheDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
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

		outPath, err := getFn(ctx, tempDir, src)
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

	if err := os.MkdirAll(destPath, 0o755); err != nil {
		return err
	}

	if err := runRsyncCmd(ctx, cachePath, os.Stdout, os.Stderr, "-aEq", "--delete", ".", destPath); err != nil {
		return err
	}

	return nil
}

func runRsyncCmd(ctx context.Context, root string, stdout io.Writer, stderr io.Writer, args ...string) error {
	cmd := exec.CommandContext(ctx, "rsync", args...)

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
