package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cert-manager/klone/pkg/cache"
	"github.com/cert-manager/klone/pkg/download/git"
	"github.com/cert-manager/klone/pkg/mod"
	"github.com/spf13/cobra"
)

func NewSyncCommand() *cobra.Command {
	cmds := &cobra.Command{
		Use:  "sync",
		Args: cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			workDirPath, err := filepath.Abs(".")
			if err != nil {
				return err
			}

			wrkDir := mod.WorkDir(workDirPath)
			if err := wrkDir.FetchTargets(
				func(_ string, _ string, src *mod.KloneSource) error {
					src.RepoPath = filepath.Join(".", filepath.Clean(filepath.Join("/", src.RepoPath)))

					if src.RepoHash == "" {
						hash, err := git.GetHash(src.RepoURL, src.RepoRef)
						if err != nil {
							return err
						}

						src.RepoHash = hash
					}

					return nil
				},
				func(target string, srcs mod.KloneFolder) error {
					if err := os.RemoveAll(filepath.Join(workDirPath, target)); err != nil {
						return err
					}

					if err := os.MkdirAll(filepath.Join(workDirPath, target), 0755); err != nil {
						return err
					}

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
		},
	}

	return cmds
}
