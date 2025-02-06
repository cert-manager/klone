package main

import (
	"context"

	"github.com/cert-manager/klone/cmd"
)

func main() {
	ctx := context.Background()

	cmd := cmd.NewCommand()

	if err := cmd.ExecuteContext(ctx); err != nil {
		panic(err)
	}
}
