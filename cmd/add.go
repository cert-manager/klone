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

package cmd

import (
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cert-manager/klone/pkg/mod"
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
