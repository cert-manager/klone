package cmd

import (
	"path/filepath"

	"github.com/cert-manager/klone/pkg/sync"
	"github.com/spf13/cobra"
)

func NewUpgradeCommand() *cobra.Command {
	cmds := &cobra.Command{
		Use:   "upgrade",
		Args:  cobra.ExactArgs(0),
		Short: "Update all hashes to the latest upstream available and sync",
		RunE: func(cmd *cobra.Command, args []string) error {
			workDirPath, err := filepath.Abs(".")
			if err != nil {
				return err
			}

			return sync.SyncFolder(cmd.Context(), workDirPath, true)
		},
	}

	return cmds
}
