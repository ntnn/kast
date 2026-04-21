// Package kubectl wraps kubectl apply, diff, and delete operations.
package kubectl

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/openapi3"
	"k8s.io/kubectl/pkg/cmd/apply"
	cmddelete "k8s.io/kubectl/pkg/cmd/delete"
	"k8s.io/kubectl/pkg/cmd/diff"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/scheme"
)

// DryRunStrategy maps string values to kubectl's DryRunStrategy type.
func parseDryRun(s string) cmdutil.DryRunStrategy {
	switch s {
	case "none", "":
		return cmdutil.DryRunNone
	case "client":
		return cmdutil.DryRunClient
	case "server":
		return cmdutil.DryRunServer
	default:
		return cmdutil.DryRunServer
	}
}

func newFactory(kubeconfig string) cmdutil.Factory { //nolint:ireturn // cmdutil.NewFactory returns interface
	flags := genericclioptions.NewConfigFlags(true)
	if kubeconfig != "" {
		flags.KubeConfig = &kubeconfig
	}
	return cmdutil.NewFactory(flags)
}

func writeTempManifest(data []byte) (string, func(), error) {
	f, err := os.CreateTemp("", "kast-manifests-*.yaml")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp file: %w", err)
	}
	cleanup := func() { _ = os.Remove(f.Name()) }

	if _, err := f.Write(data); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("writing temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("closing temp file: %w", err)
	}
	return f.Name(), cleanup, nil
}

type applyDeps struct {
	factory       cmdutil.Factory
	dynamicClient dynamic.Interface
	mapper        meta.RESTMapper
	ns            string
	enforceNs     bool
	streams       genericiooptions.IOStreams
}

func newApplyDeps(kubeconfig string, stdout, stderr io.Writer) (*applyDeps, error) {
	f := newFactory(kubeconfig)

	dynamicClient, err := f.DynamicClient()
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}

	mapper, err := f.ToRESTMapper()
	if err != nil {
		return nil, fmt.Errorf("creating REST mapper: %w", err)
	}

	ns, enforceNs, err := f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return nil, fmt.Errorf("getting namespace: %w", err)
	}

	return &applyDeps{
		factory:       f,
		dynamicClient: dynamicClient,
		mapper:        mapper,
		ns:            ns,
		enforceNs:     enforceNs,
		streams:       genericiooptions.IOStreams{In: &bytes.Buffer{}, Out: stdout, ErrOut: stderr},
	}, nil
}

// Apply performs a server-side apply with applyset pruning.
func Apply(
	_ context.Context, stdout, stderr io.Writer,
	kubeconfig string, manifests []byte, applysetName, dryRun string,
) error {
	if err := os.Setenv("KUBECTL_APPLYSET", "true"); err != nil {
		return fmt.Errorf("setting KUBECTL_APPLYSET env: %w", err)
	}

	tmpFile, cleanup, err := writeTempManifest(manifests)
	if err != nil {
		return err
	}
	defer cleanup()

	d, err := newApplyDeps(kubeconfig, stdout, stderr)
	if err != nil {
		return err
	}

	dryRunStrategy := parseDryRun(dryRun)

	printFlags := genericclioptions.NewPrintFlags("created").WithTypeSetter(scheme.Scheme)
	toPrinter := func(operation string) (printers.ResourcePrinter, error) {
		printFlags.NamePrintFlags.Operation = operation
		cmdutil.PrintFlagsWithDryRunStrategy(printFlags, dryRunStrategy)
		return printFlags.ToPrinter()
	}

	parentRef, err := apply.ParseApplySetParentRef(applysetName, d.mapper)
	if err != nil {
		return fmt.Errorf("parsing applyset parent ref: %w", err)
	}
	if parentRef.IsNamespaced() {
		parentRef.Namespace = d.ns
	}

	tooling := apply.ApplySetTooling{Name: "kast", Version: "v0.0.0"}
	restClient, err := d.factory.UnstructuredClientForMapping(parentRef.RESTMapping)
	if err != nil {
		return fmt.Errorf("creating REST client for applyset: %w", err)
	}
	applySet := apply.NewApplySet(parentRef, tooling, d.mapper, restClient)

	deleteOpts := &cmddelete.DeleteOptions{
		FilenameOptions: resource.FilenameOptions{
			Filenames: []string{tmpFile},
		},
		CascadingStrategy: metav1.DeletePropagationBackground,
		GracePeriod:       -1,
		IOStreams:         d.streams,
	}

	validator, err := d.factory.Validator(metav1.FieldValidationStrict)
	if err != nil {
		return fmt.Errorf("creating validator: %w", err)
	}

	o := &apply.ApplyOptions{
		PrintFlags:          printFlags,
		ToPrinter:           toPrinter,
		ServerSideApply:     true,
		ForceConflicts:      true,
		FieldManager:        "kast",
		DryRunStrategy:      dryRunStrategy,
		Prune:               true,
		Overwrite:           true,
		OpenAPIPatch:        true,
		Recorder:            genericclioptions.NoopRecorder{},
		Namespace:           d.ns,
		EnforceNamespace:    d.enforceNs,
		Validator:           validator,
		ValidationDirective: metav1.FieldValidationStrict,
		Builder:             d.factory.NewBuilder(),
		Mapper:              d.mapper,
		DynamicClient:       d.dynamicClient,
		IOStreams:           d.streams,
		VisitedUids:         sets.New[types.UID](),
		VisitedNamespaces:   sets.New[string](),
		ApplySet:            applySet,
		DeleteOptions:       deleteOpts,
	}

	o.PostProcessorFn = o.PrintAndPrunePostProcessor()

	if err := o.Validate(); err != nil {
		return fmt.Errorf("validating apply options: %w", err)
	}

	return o.Run()
}

// Diff performs a server-side diff of manifests against the cluster.
func Diff(_ context.Context, stdout, stderr io.Writer, kubeconfig string, manifests []byte) error {
	tmpFile, cleanup, err := writeTempManifest(manifests)
	if err != nil {
		return err
	}
	defer cleanup()

	f := newFactory(kubeconfig)

	streams := genericiooptions.IOStreams{In: &bytes.Buffer{}, Out: stdout, ErrOut: stderr}

	ns, enforceNs, err := f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return fmt.Errorf("getting namespace: %w", err)
	}

	dynamicClient, err := f.DynamicClient()
	if err != nil {
		return fmt.Errorf("creating dynamic client: %w", err)
	}

	openAPIGetter := f
	openAPIV3Client, err := f.OpenAPIV3Client()
	if err != nil {
		return fmt.Errorf("getting OpenAPI v3 client: %w", err)
	}

	o := diff.NewDiffOptions(streams)
	o.ServerSideApply = true
	o.ForceConflicts = true
	o.FieldManager = "kast"
	o.CmdNamespace = ns
	o.EnforceNamespace = enforceNs
	o.DynamicClient = dynamicClient
	o.OpenAPIGetter = openAPIGetter
	o.OpenAPIV3Root = openapi3.NewRoot(openAPIV3Client)
	o.Builder = f.NewBuilder()
	o.FilenameOptions = resource.FilenameOptions{
		Filenames: []string{tmpFile},
	}

	return o.Run()
}

// Delete removes resources from the cluster.
func Delete(_ context.Context, stdout, stderr io.Writer, kubeconfig string, manifests []byte, dryRun string) error {
	tmpFile, cleanup, err := writeTempManifest(manifests)
	if err != nil {
		return err
	}
	defer cleanup()

	f := newFactory(kubeconfig)

	dynamicClient, err := f.DynamicClient()
	if err != nil {
		return fmt.Errorf("creating dynamic client: %w", err)
	}

	mapper, err := f.ToRESTMapper()
	if err != nil {
		return fmt.Errorf("creating REST mapper: %w", err)
	}

	ns, enforceNs, err := f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return fmt.Errorf("getting namespace: %w", err)
	}

	streams := genericiooptions.IOStreams{In: &bytes.Buffer{}, Out: stdout, ErrOut: stderr}

	result := f.NewBuilder().
		Unstructured().
		ContinueOnError().
		NamespaceParam(ns).DefaultNamespace().
		FilenameParam(enforceNs, &resource.FilenameOptions{
			Filenames: []string{tmpFile},
		}).
		Flatten().
		Do()

	o := &cmddelete.DeleteOptions{
		FilenameOptions: resource.FilenameOptions{
			Filenames: []string{tmpFile},
		},
		CascadingStrategy: metav1.DeletePropagationForeground,
		WaitForDeletion:   true,
		IgnoreNotFound:    true,
		GracePeriod:       -1,
		DryRunStrategy:    parseDryRun(dryRun),
		DynamicClient:     dynamicClient,
		Mapper:            mapper,
		Result:            result,
		IOStreams:         streams,
		WarningPrinter:    printers.NewWarningPrinter(stderr, printers.WarningPrinterOptions{}),
	}

	_ = ns
	_ = enforceNs

	return o.DeleteResult(result)
}
