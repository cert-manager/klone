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
	// assertNoSymlinkInTree treats workDirPath as a trusted root and does
	// not inspect it. Resolve symlinks once up-front so a caller invoking
	// klone from inside a symlinked path cannot shift the trust boundary
	// above the intended directory (VC-53816).
	resolved, err := filepath.EvalSymlinks(workDirPath)
	if err != nil {
		return fmt.Errorf("failed to resolve workDir %q: %w", workDirPath, err)
	}
	workDirPath = resolved

	workDir := mod.WorkDir(workDirPath)
	if err := workDir.FetchTargets(
		func(_ string, _ string, src *mod.KloneSource) error {
			if err := mod.ValidateRepoURL(src.RepoURL); err != nil {
				return err
			}

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
			targetSegments, err := splitTargetName(target)
			if err != nil {
				return err
			}

			parsedSegments := make([][]string, len(srcs))
			folders := newTreeNode()
			for i, src := range srcs {
				segments, err := splitFolderName(src.FolderName)
				if err != nil {
					return err
				}
				parsedSegments[i] = segments
				folders.Add(segments...)
			}

			// Check the target even when srcs is empty; Cleanup still runs.
			if err := assertNoSymlinkInTree(workDirPath, targetSegments); err != nil {
				return err
			}
			targetRoot := filepath.Join(append([]string{workDirPath}, targetSegments...)...)
			if err := os.MkdirAll(targetRoot, 0755); err != nil {
				return err
			}

			// 1) Remove all folders that are not defined in srcs
			if err := folders.Cleanup(targetRoot); err != nil {
				return err
			}

			// 2) Sync all folders with cached files
			for i, src := range srcs {
				if err := assertNoSymlinkInTree(targetRoot, parsedSegments[i]); err != nil {
					return err
				}
				dest := filepath.Join(append([]string{targetRoot}, parsedSegments[i]...)...)
				if err := cache.CloneWithCache(ctx, dest, src.KloneSource, git.Get); err != nil {
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

func splitFolderName(folderName string) ([]string, error) {
	return splitManifestPath("folder_name", folderName)
}

func splitTargetName(targetName string) ([]string, error) {
	return splitManifestPath("target", targetName)
}

// splitManifestPath parses manifest-controlled paths consistently across OSes.
func splitManifestPath(fieldName, value string) ([]string, error) {
	if value == "" {
		return nil, fmt.Errorf("invalid %s %q: empty", fieldName, value)
	}
	if hasWindowsDrivePrefix(value) {
		return nil, fmt.Errorf("invalid %s %q: Windows volume prefix is not allowed", fieldName, value)
	}
	if filepath.IsAbs(value) || filepath.VolumeName(value) != "" {
		return nil, fmt.Errorf("invalid %s %q: absolute paths and volume prefixes are not allowed", fieldName, value)
	}
	normalised := strings.ReplaceAll(value, `\`, "/")
	segments := strings.Split(normalised, "/")
	for _, seg := range segments {
		if seg == "" || seg == "." || seg == ".." {
			return nil, fmt.Errorf("invalid %s %q: empty or traversal segment %q", fieldName, value, seg)
		}
	}
	return segments, nil
}

// assertNoSymlinkInTree rejects existing symlinks along a destination path.
func assertNoSymlinkInTree(root string, segments []string) error {
	cur := root
	for _, seg := range segments {
		cur = filepath.Join(cur, seg)
		info, err := os.Lstat(cur)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to traverse symlink at %q (VC-53818)", cur)
		}
	}
	return nil
}

// hasWindowsDrivePrefix catches drive-qualified paths on every GOOS.
func hasWindowsDrivePrefix(s string) bool {
	if len(s) < 2 || s[1] != ':' {
		return false
	}
	c := s[0]
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
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

	if info, err := os.Lstat(root); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	} else if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("treeNode.Cleanup: refusing to operate on symlinked root %q (VC-53818)", root)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
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

		if err := os.RemoveAll(filepath.Join(root, entryName)); err != nil {
			return err
		}
	}

	for name, node := range tn.children {
		if name == "" || name == "." || name == ".." {
			return fmt.Errorf("treeNode.Cleanup: refusing to descend into unsafe child name %q", name)
		}
		if err := node.Cleanup(filepath.Join(root, name)); err != nil {
			return err
		}
	}

	return nil
}
