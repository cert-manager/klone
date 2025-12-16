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
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cmds := &cobra.Command{
		Use:   "klone",
		Short: "Clone folders from a git repo locally",
		Long: `Clone folders from an upstream git repo locally

Klone takes a config file as input and copies folders from listed upstream
git repositories to the local directory.

To get started, run "klone init" which will create a barebones klone.yaml file
which does nothing.

To add a target for kloning, use "klone add", e.g.:

klone add example myfolder https://github.com/cert-manager/community.git logo main

This will add an entry to klone.yaml which fetches the latest cert-manager
logo from the community repo and stores it in example/myfolder.

Finally, we can run "klone sync" to actually perform the checkout. If you ran
the "klone add" command above, you'll see that the "example/myfolder" directory
has been populated from the remote git repository.

If there's an upstream update later, "klone upgrade" will fetch the latest
revision for the upstream and check out the results locally.`,
	}

	cmds.AddCommand(NewInitCommand())
	cmds.AddCommand(NewSyncCommand())
	cmds.AddCommand(NewAddCommand())
	cmds.AddCommand(NewUpgradeCommand())

	return cmds
}
