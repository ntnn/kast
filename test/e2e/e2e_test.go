package e2e

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/kcp-dev/multicluster-provider/envtest"
	"github.com/ntnn/kast/pkg/kubectl"
	"github.com/ntnn/kast/pkg/kustomize"
	"github.com/ntnn/kast/pkg/order"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var (
	kubeconfigPath string
	dynClient      dynamic.Interface
)

func TestMain(m *testing.M) {
	env := &envtest.Environment{}

	cfg, err := env.Start()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start envtest: %v\n", err)
		os.Exit(1)
	}

	wsCfg := rest.CopyConfig(cfg)
	wsCfg.Host += "/clusters/root"

	kubeconfigPath, err = writeKubeconfig(wsCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to write kubeconfig: %v\n", err)
		_ = env.Stop()
		os.Exit(1)
	}

	dynClient, err = dynamic.NewForConfig(wsCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create dynamic client: %v\n", err)
		_ = env.Stop()
		os.Exit(1)
	}

	code := m.Run()

	_ = os.Remove(kubeconfigPath)
	_ = env.Stop()
	os.Exit(code)
}

func writeKubeconfig(cfg *rest.Config) (string, error) {
	kubeconfig := clientcmdapi.NewConfig()
	kubeconfig.Clusters["envtest"] = &clientcmdapi.Cluster{
		Server:                   cfg.Host,
		CertificateAuthorityData: cfg.CAData,
	}
	kubeconfig.AuthInfos["envtest"] = &clientcmdapi.AuthInfo{
		ClientCertificateData: cfg.CertData,
		ClientKeyData:         cfg.KeyData,
	}
	kubeconfig.Contexts["envtest"] = &clientcmdapi.Context{
		Cluster:   "envtest",
		AuthInfo:  "envtest",
		Namespace: "default",
	}
	kubeconfig.CurrentContext = "envtest"

	f, err := os.CreateTemp("", "kast-e2e-kubeconfig-*.yaml")
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	data, err := clientcmd.Write(*kubeconfig)
	if err != nil {
		return "", err
	}
	if _, err := f.Write(data); err != nil {
		return "", err
	}
	return f.Name(), nil
}

func testdataDir(name string) string {
	return filepath.Join("testdata", name)
}

var (
	configMapGVR = schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	secretGVR    = schema.GroupVersionResource{Version: "v1", Resource: "secrets"}
)

func getResource(t *testing.T, gvr schema.GroupVersionResource, name string) *unstructured.Unstructured {
	t.Helper()
	obj, err := dynClient.Resource(gvr).Namespace("default").Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return nil
	}
	return obj
}

func renderAndSort(t *testing.T, dir string) []byte {
	t.Helper()
	manifests, err := kustomize.Render(testdataDir(dir))
	require.NoError(t, err)
	sorted, err := order.Sort(manifests)
	require.NoError(t, err)
	return sorted
}

func renderAndSortReverse(t *testing.T, dir string) []byte {
	t.Helper()
	manifests, err := kustomize.Render(testdataDir(dir))
	require.NoError(t, err)
	sorted, err := order.SortReverse(manifests)
	require.NoError(t, err)
	return sorted
}

func TestApply(t *testing.T) { //nolint:paralleltest // e2e tests share cluster state
	manifests := renderAndSort(t, "simple")

	var stdout, stderr bytes.Buffer
	err := kubectl.Apply(context.Background(), &stdout, &stderr, kubeconfigPath, manifests, "e2e-simple", "none")
	require.NoError(t, err, "stderr: %s", stderr.String())

	obj := getResource(t, configMapGVR, "e2e-simple")
	require.NotNil(t, obj, "ConfigMap e2e-simple should exist after apply")
	assert.Equal(t, "value", obj.Object["data"].(map[string]any)["key"])
}

func TestApplyIdempotent(t *testing.T) { //nolint:paralleltest // e2e tests share cluster state
	manifests := renderAndSort(t, "simple")

	var stdout, stderr bytes.Buffer
	err := kubectl.Apply(context.Background(), &stdout, &stderr, kubeconfigPath, manifests, "e2e-idempotent", "none")
	require.NoError(t, err, "first apply stderr: %s", stderr.String())

	stdout.Reset()
	stderr.Reset()
	err = kubectl.Apply(context.Background(), &stdout, &stderr, kubeconfigPath, manifests, "e2e-idempotent", "none")
	require.NoError(t, err, "second apply stderr: %s", stderr.String())
}

func TestApplyPruning(t *testing.T) { //nolint:paralleltest // e2e tests share cluster state
	// Apply the "before" set: ConfigMap + Secret
	before := renderAndSort(t, "pruning-before")
	var stdout, stderr bytes.Buffer
	err := kubectl.Apply(context.Background(), &stdout, &stderr, kubeconfigPath, before, "e2e-prune", "none")
	require.NoError(t, err, "before apply stderr: %s", stderr.String())

	// Verify both exist
	require.NotNil(t, getResource(t, configMapGVR, "e2e-prune"), "ConfigMap should exist")
	require.NotNil(t, getResource(t, secretGVR, "e2e-prune-secret"), "Secret should exist")

	// Apply the "after" set: only ConfigMap (Secret removed)
	after := renderAndSort(t, "pruning-after")
	stdout.Reset()
	stderr.Reset()
	err = kubectl.Apply(context.Background(), &stdout, &stderr, kubeconfigPath, after, "e2e-prune", "none")
	require.NoError(t, err, "after apply stderr: %s", stderr.String())

	// ConfigMap should still exist with updated data
	cm := getResource(t, configMapGVR, "e2e-prune")
	require.NotNil(t, cm, "ConfigMap should still exist after pruning")
	assert.Equal(t, "after", cm.Object["data"].(map[string]any)["key"])

	// Secret should have been pruned
	assert.Nil(t, getResource(t, secretGVR, "e2e-prune-secret"), "Secret should be pruned")
}

func TestDiff(t *testing.T) { //nolint:paralleltest // e2e tests share cluster state
	// First apply so there's a baseline
	manifests := renderAndSort(t, "simple")
	var applyOut, applyErr bytes.Buffer
	err := kubectl.Apply(context.Background(), &applyOut, &applyErr, kubeconfigPath, manifests, "e2e-diff", "none")
	require.NoError(t, err, "apply stderr: %s", applyErr.String())

	// Diff the same manifests — diff may return exit 1 due to SSA metadata
	var stdout, stderr bytes.Buffer
	_ = kubectl.Diff(context.Background(), &stdout, &stderr, kubeconfigPath, manifests)

	// Now diff modified manifests against cluster
	modified := bytes.ReplaceAll(manifests, []byte("key: value"), []byte("key: modified"))
	stdout.Reset()
	stderr.Reset()
	_ = kubectl.Diff(context.Background(), &stdout, &stderr, kubeconfigPath, modified)
	// diff returns exit code 1 when there are differences
	// The stdout should contain diff output with our change
	assert.Contains(t, stdout.String(), "modified", "diff output should show the change")
}

func TestDelete(t *testing.T) { //nolint:paralleltest // e2e tests share cluster state
	// Apply first
	manifests := renderAndSort(t, "simple")
	var stdout, stderr bytes.Buffer
	err := kubectl.Apply(context.Background(), &stdout, &stderr, kubeconfigPath, manifests, "e2e-delete", "none")
	require.NoError(t, err, "apply stderr: %s", stderr.String())

	require.NotNil(t, getResource(t, configMapGVR, "e2e-simple"), "ConfigMap should exist before delete")

	// Delete
	deleteManifests := renderAndSortReverse(t, "simple")
	stdout.Reset()
	stderr.Reset()
	err = kubectl.Delete(context.Background(), &stdout, &stderr, kubeconfigPath, deleteManifests, "none")
	require.NoError(t, err, "delete stderr: %s", stderr.String())

	assert.Nil(t, getResource(t, configMapGVR, "e2e-simple"), "ConfigMap should not exist after delete")
}
