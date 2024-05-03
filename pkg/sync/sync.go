package sync

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cert-manager/klone/pkg/cache"
	"github.com/cert-manager/klone/pkg/download/git"
	"github.com/cert-manager/klone/pkg/mod"
)

func SyncFolder(workDirPath string, forceUpgrade bool) error {
	wrkDir := mod.WorkDir(workDirPath)
	if err := wrkDir.FetchTargets(
		func(_ string, _ string, src *mod.KloneSource) error {
			src.RepoPath = filepath.Join(".", filepath.Clean(filepath.Join("/", src.RepoPath)))

			if src.RepoHash == "" || forceUpgrade {
				hash, err := git.GetHash(src.RepoURL, src.RepoRef)
				if err != nil {
					return err
				}

				src.RepoHash = hash
			}

			return nil
		},
		func(target string, srcs mod.KloneFolder) error {
			folders := newTreeNode()
			for _, src := range srcs {
				folders.Add(filepath.SplitList(src.FolderName)...)
			}

			if err := os.MkdirAll(filepath.Join(workDirPath, target), 0755); err != nil {
				return err
			}

			// 1) Remove all folders that are not defined in srcs
			if err := folders.Cleanup(filepath.Join(workDirPath, target)); err != nil {
				return err
			}

			// 2) Sync all folders with cached files
			for _, src := range srcs {
				if err := cache.CloneWithCache(filepath.Join(workDirPath, target, src.FolderName), src.KloneSource, git.Get); err != nil {
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
		if err := node.Cleanup(filepath.Join(root, name)); err != nil {
			return err
		}
	}

	return nil
}
