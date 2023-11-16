package main

import (
	"github.com/go418/klone/cmd"
)

func main() {
	cmd := cmd.NewCommand()

	if err := cmd.Execute(); err != nil {
		panic(err)
	}
}
