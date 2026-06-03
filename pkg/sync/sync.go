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

package sync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cert-manager/klone/pkg/cache"
	"github.com/cert-manager/klone/pkg/download/git"
	"github.com/cert-manager/klone/pkg/mod"
)

func SyncFolder(ctx context.Context, workDirPath string, forceUpgrade bool) error {
	// AssertNoSymlinkInSubpath treats workDirPath as a trusted root and
	// does not inspect it. Resolve symlinks once up-front so a caller
	// invoking klone from inside a symlinked path cannot shift the trust
	// boundary above the intended directory.
	resolved, err := filepath.EvalSymlinks(workDirPath)
	if err != nil {
		return fmt.Errorf("failed to resolve workDir %q: %w", workDirPath, err)
	}
	workDirPath = resolved

	workDir := mod.WorkDir(workDirPath)
	if err := workDir.FetchTargets(
		func(_ string, _ string, src *mod.KloneSource) error {
			src.RepoPath = filepath.Join(".", filepath.Clean(filepath.Join("/", src.RepoPath)))

			if src.RepoHash == "" || forceUpgrade {
				hash, err := git.GetHash(ctx, src.RepoURL, src.RepoRef)
				if err != nil {
					return err
				}

				src.RepoHash = hash
			}

			return nil
		},
		func(target string, srcs mod.KloneFolder) error {
			canonical := make([]string, len(srcs))
			folders := newTreeNode()
			for i, src := range srcs {
				segments, err := splitFolderName(src.FolderName)
				if err != nil {
					return err
				}
				canonical[i] = filepath.Join(segments...)
				folders.Add(segments...)
			}

			if err := cache.AssertNoSymlinkInSubpath(workDirPath, target); err != nil {
				return err
			}

			if err := os.MkdirAll(filepath.Join(workDirPath, target), 0755); err != nil {
				return err
			}

			targetRoot := filepath.Join(workDirPath, target)

			// Pre-flight: walk every prefix of each folder_name before Cleanup
			// runs. Cleanup recurses via os.ReadDir, which follows symlinks,
			// so a pre-planted symlink at any intermediate directory (e.g.
			// workDir/vendored/a -> /etc, left over from a compromised state
			// prior to this fix) would otherwise have its target's entries
			// deleted by os.RemoveAll on the first post-fix run. The same
			// walk also covers the in-tree (safe-by-rsync) symlink that an
			// earlier iteration may have planted to redirect later writes.
			for i := range srcs {
				if err := cache.AssertNoSymlinkInSubpath(targetRoot, canonical[i]); err != nil {
					return err
				}
			}

			// 1) Remove all folders that are not defined in srcs
			if err := folders.Cleanup(targetRoot); err != nil {
				return err
			}

			// 2) Sync all folders with cached files
			for i, src := range srcs {
				if err := cache.CloneWithCache(ctx, filepath.Join(targetRoot, canonical[i]), src.KloneSource, git.Get); err != nil {
					return err
				}
			}

			return nil
		},
	); err != nil {
		return fmt.Errorf("failed to fetch targets: %w", err)
	}

	if err := cache.CleanupOldCacheItems(); err != nil {
		return fmt.Errorf("failed to cleanup old cache items: %w", err)
	}

	return nil

}

type treeNode struct {
	isLeaf   bool
	children map[string]*treeNode
}

func newTreeNode() *treeNode {
	return &treeNode{
		isLeaf:   false,
		children: make(map[string]*treeNode),
	}
}

func (tn *treeNode) Add(pathSegments ...string) {
	if len(pathSegments) == 0 {
		tn.isLeaf = true
		return
	}

	if _, ok := tn.children[pathSegments[0]]; !ok {
		tn.children[pathSegments[0]] = newTreeNode()
	}

	tn.children[pathSegments[0]].Add(pathSegments[1:]...)
}

func (tn treeNode) Cleanup(root string) error {
	if tn.isLeaf {
		return nil
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		// Once splitFolderName produces multi-segment trees, Cleanup
		// recurses into intermediate dirs that may not exist on first
		// sync (the previous SplitList bug always produced flat trees,
		// hiding this). Treat missing as "no entries to clean".
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		entryName := entry.Name()
		if _, ok := tn.children[entryName]; ok {
			continue
		}

		entryPath := filepath.Join(root, entryName)
		// Re-Lstat the entry just before RemoveAll. If an attacker with
		// concurrent write access swapped a regular directory for a symlink
		// between ReadDir and now, RemoveAll's behaviour on the symlinked
		// path is platform-dependent. Refusing closes the window.
		entryInfo, err := os.Lstat(entryPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if entryInfo.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("treeNode.Cleanup: refusing to remove symlink entry %q (VC-53818)", entryPath)
		}

		if err := os.RemoveAll(entryPath); err != nil {
			return err
		}
	}

	for name, node := range tn.children {
		if err := node.Cleanup(filepath.Join(root, name)); err != nil {
			return err
		}
	}

	return nil
}

// splitFolderName parses a manifest folder_name into path segments. The
// previous implementation used filepath.SplitList, which splits on the
// PATH env separator (':' on Unix, ';' on Windows), not the path-component
// separator — so a manifest entry like "..:..:.." became three ".."
// segments fed into treeNode.Cleanup, which then escaped the target tree.
// This splitter uses the path-component separator and rejects every
// segment shape that could traverse outside the target.
func splitFolderName(folderName string) ([]string, error) {
	if folderName == "" {
		return nil, fmt.Errorf("invalid folder_name %q: empty", folderName)
	}
	if hasWindowsDrivePrefix(folderName) {
		return nil, fmt.Errorf("invalid folder_name %q: Windows volume prefix is not allowed", folderName)
	}
	if filepath.IsAbs(folderName) || filepath.VolumeName(folderName) != "" {
		return nil, fmt.Errorf("invalid folder_name %q: absolute paths and volume prefixes are not allowed", folderName)
	}
	// strings.ReplaceAll, not filepath.ToSlash: ToSlash is a no-op on Unix,
	// which would let a Linux-authored manifest smuggle "a\b" past
	// validation for a Windows victim where it would resolve as "a/b".
	normalised := strings.ReplaceAll(folderName, `\`, "/")
	segments := strings.Split(normalised, "/")
	for _, seg := range segments {
		if seg == "" || seg == "." || seg == ".." {
			return nil, fmt.Errorf("invalid folder_name %q: empty or traversal segment %q", folderName, seg)
		}
	}
	return segments, nil
}

// hasWindowsDrivePrefix catches drive-qualified paths on every GOOS;
// filepath.VolumeName only recognises the shape when GOOS=windows.
func hasWindowsDrivePrefix(s string) bool {
	if len(s) < 2 || s[1] != ':' {
		return false
	}
	c := s[0]
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}
