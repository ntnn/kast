// kast renders kustomize directories and applies/diffs/deletes them.
package main

import (
	"context"
	"flag"
	"io"

	"codeberg.org/ntnn/mindl/pkg/simplcli"
	"github.com/ntnn/kast/pkg/kast"
)

func main() {
	kaster := kast.New()
	fs := flag.NewFlagSet("kast", flag.ExitOnError)
	kaster.RegisterFlags(fs)

	cli := simplcli.SimplCLI{
		SubCmds: map[string]simplcli.SubCmd{
			"apply": {
				Runner: wrapper(kaster.Apply),
				Doc:    "Render and apply kustomize directories with server-side apply and applyset pruning",
			},
			"diff": {
				Runner: wrapper(kaster.Diff),
				Doc:    "Render and diff kustomize directories against the cluster",
			},
			"delete": {
				Runner: wrapper(kaster.Delete),
				Doc:    "Render and delete resources from kustomize directories",
			},
		},
	}

	simplcli.EntrypointWithFlags(cli, fs)
}

type action func(context.Context, string) error

func wrapper(fn action) simplcli.Runner {
	return func(ctx context.Context, _, _ io.Writer, args []string) error {
		// TODO: just for now. Ideally Kaster or a wrapper around it
		// could do multiple dirs in parallel.
		for _, arg := range args {
			if err := fn(ctx, arg); err != nil {
				return err
			}
		}
		return nil
	}
}
