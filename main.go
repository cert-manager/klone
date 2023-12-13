package main

import (
	"github.com/cert-manager/klone/cmd"
)

func main() {
	cmd := cmd.NewCommand()

	if err := cmd.Execute(); err != nil {
		panic(err)
	}
}
