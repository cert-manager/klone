package cmd

import (
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cmds := &cobra.Command{
		Use: "klone",
	}

	cmds.AddCommand(NewInitCommand())
	cmds.AddCommand(NewSyncCommand())
	cmds.AddCommand(NewAddCommand())
	cmds.AddCommand(NewUpgradeCommand())

	return cmds
}
