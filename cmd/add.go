package cmd

import (
	"path/filepath"

	"github.com/cert-manager/klone/pkg/mod"

	"github.com/spf13/cobra"
)

func NewAddCommand() *cobra.Command {
	cmds := &cobra.Command{
		Use:   "add [DST_PATH] [DST_FOLDER_NAME] [REPO_URL] [REPO_REF] [REPO_FOLDER]",
		Short: "Add a new target to sync from an upstream git repository",
		Example: `Sync the 'logo' directory from the main branch of the cert-manager
community repository to the local directory ./a/b

klone add a b https://github.com/cert-manager/community.git main logo`,
		Args: cobra.ExactArgs(5),
		RunE: func(cmd *cobra.Command, args []string) error {
			workDirPath, err := filepath.Abs(".")
			if err != nil {
				return err
			}

			wrkDir := mod.WorkDir(workDirPath)

			dstPath := args[0]
			dstFolderName := args[1]
			repoURL := args[2]
			ref := args[3]
			srcFolder := args[4]

			return wrkDir.AddTarget(dstPath, dstFolderName, mod.KloneSource{
				RepoURL:  repoURL,
				RepoRef:  ref,
				RepoPath: srcFolder,
			})
		},
	}

	return cmds
}
