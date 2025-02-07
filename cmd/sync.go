package cmd

import (
	"path/filepath"

	"github.com/cert-manager/klone/pkg/sync"
	"github.com/spf13/cobra"
)

func NewSyncCommand() *cobra.Command {
	cmds := &cobra.Command{
		Use:   "sync",
		Short: "Ensure the local state of targets matches upstream",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			workDirPath, err := filepath.Abs(".")
			if err != nil {
				return err
			}

			return sync.SyncFolder(cmd.Context(), workDirPath, false)
		},
	}

	return cmds
}
