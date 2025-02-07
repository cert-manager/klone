package cmd

import (
	"path/filepath"

	"github.com/cert-manager/klone/pkg/mod"

	"github.com/spf13/cobra"
)

func NewAddCommand() *cobra.Command {
	cmds := &cobra.Command{
		Use:   "add dst_path dst_folder_name repo_url repo_path repo_ref [repo_hash]",
		Short: "Add a new target to sync from an upstream git repository",
		Example: `Sync the 'logo' directory from the main branch of the cert-manager
community repository to the local directory ./a/b

  klone add a b https://github.com/cert-manager/community.git logo main
    or with pinned commit hash:
  klone add a b https://github.com/cert-manager/community.git logo main 9f0ea0341816665feadcdcfb7744f4245604ab28`,
		Args: cobra.RangeArgs(5, 6),
		RunE: func(cmd *cobra.Command, args []string) error {
			workDirPath, err := filepath.Abs(".")
			if err != nil {
				return err
			}

			workDir := mod.WorkDir(workDirPath)

			dstPath := args[0]
			dstFolderName := args[1]
			repoURL := args[2]
			repoPath := args[3]
			repoRef := args[4]

			repoHash := ""
			if len(args) == 6 {
				repoHash = args[5]
			}

			return workDir.AddTarget(dstPath, dstFolderName, mod.KloneSource{
				RepoURL:  repoURL,
				RepoPath: repoPath,
				RepoRef:  repoRef,
				RepoHash: repoHash,
			})
		},
	}

	return cmds
}
