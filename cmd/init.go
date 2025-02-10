package cmd

import (
	"path/filepath"

	"github.com/cert-manager/klone/pkg/mod"

	"github.com/spf13/cobra"
)

func NewInitCommand() *cobra.Command {
	cmds := &cobra.Command{
		Use:   "init",
		Short: "Initialise a new klone.yaml file and exit",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			workDirPath, err := filepath.Abs(".")
			if err != nil {
				return err
			}

			workDir := mod.WorkDir(workDirPath)
			return workDir.Init()
		},
	}

	return cmds
}
