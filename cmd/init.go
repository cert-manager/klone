package cmd

import (
	"path/filepath"

	"github.com/go418/klone/pkg/mod"
	"github.com/spf13/cobra"
)

func NewInitCommand() *cobra.Command {
	cmds := &cobra.Command{
		Use:  "init",
		Args: cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			workDirPath, err := filepath.Abs(".")
			if err != nil {
				return err
			}

			wrkDir := mod.WorkDir(workDirPath)
			return wrkDir.Init()
		},
	}

	return cmds
}
