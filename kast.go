// kast renders kustomize directories and applies/diffs/deletes them.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/ntnn/kast/pkg/kubectl"
	"github.com/ntnn/kast/pkg/kustomize"
	"github.com/ntnn/kast/pkg/order"
	"github.com/ntnn/mindl/pkg/simplcli"
)

func main() {
	simplcli.Entrypoint(simplcli.SimplCLI{
		SubCmds: map[string]simplcli.SubCmd{
			"apply": {
				Runner: makeRunner(runApply),
				Doc:    "Render and apply kustomize directories with server-side apply and applyset pruning",
			},
			"diff": {
				Runner: makeRunner(runDiff),
				Doc:    "Render and diff kustomize directories against the cluster",
			},
			"delete": {
				Runner: makeRunner(runDelete),
				Doc:    "Render and delete resources from kustomize directories",
			},
			"destroy": {
				Runner: makeRunner(runDelete),
				Doc:    "Alias for delete",
			},
		},
	})
}

type flags struct {
	dryRun     string
	kubeconfig string
}

func parseFlags(args []string) (flags, []string, error) {
	fs := flag.NewFlagSet("kast", flag.ContinueOnError)
	var f flags
	fs.StringVar(&f.dryRun, "dry-run", "server", "Dry-run strategy: none, client, server")
	fs.StringVar(&f.kubeconfig, "kubeconfig", "", "Path to kubeconfig file")
	if err := fs.Parse(args); err != nil {
		return flags{}, nil, err
	}
	return f, fs.Args(), nil
}

type actionFunc func(ctx context.Context, stdout, stderr io.Writer, f flags, dir string) error

func makeRunner(action actionFunc) simplcli.Runner {
	return func(ctx context.Context, stdout, stderr io.Writer, args []string) error {
		f, dirs, err := parseFlags(args)
		if err != nil {
			return err
		}
		if len(dirs) == 0 {
			return errors.New("at least one kustomize directory is required")
		}
		for _, dir := range dirs {
			if err := action(ctx, stdout, stderr, f, dir); err != nil {
				return fmt.Errorf("%s: %w", dir, err)
			}
		}
		return nil
	}
}

func runApply(ctx context.Context, stdout, stderr io.Writer, f flags, dir string) error {
	manifests, err := kustomize.Render(dir)
	if err != nil {
		return fmt.Errorf("rendering: %w", err)
	}

	sorted, err := order.Sort(manifests)
	if err != nil {
		return fmt.Errorf("sorting: %w", err)
	}

	return kubectl.Apply(ctx, stdout, stderr, f.kubeconfig, sorted, dir, f.dryRun)
}

func runDiff(ctx context.Context, stdout, stderr io.Writer, f flags, dir string) error {
	manifests, err := kustomize.Render(dir)
	if err != nil {
		return fmt.Errorf("rendering: %w", err)
	}

	sorted, err := order.Sort(manifests)
	if err != nil {
		return fmt.Errorf("sorting: %w", err)
	}

	return kubectl.Diff(ctx, stdout, stderr, f.kubeconfig, sorted)
}

func runDelete(ctx context.Context, stdout, stderr io.Writer, f flags, dir string) error {
	manifests, err := kustomize.Render(dir)
	if err != nil {
		return fmt.Errorf("rendering: %w", err)
	}

	sorted, err := order.SortReverse(manifests)
	if err != nil {
		return fmt.Errorf("sorting: %w", err)
	}

	return kubectl.Delete(ctx, stdout, stderr, f.kubeconfig, sorted, f.dryRun)
}
