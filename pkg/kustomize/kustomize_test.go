package kustomize

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	kustomization := `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - configmap.yaml
`
	configmap := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  key: value
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "kustomization.yaml"), []byte(kustomization), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "configmap.yaml"), []byte(configmap), 0o600))

	result, err := Render(dir)
	require.NoError(t, err)
	assert.Contains(t, string(result), "test-config")
	assert.Contains(t, string(result), "kind: ConfigMap")
}

func TestRenderInvalidPath(t *testing.T) {
	t.Parallel()

	_, err := Render("/nonexistent/path")
	require.Error(t, err)
}

func TestRenderMultipleResources(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	kustomization := `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - resources.yaml
`
	resources := `apiVersion: v1
kind: ConfigMap
metadata:
  name: config-a
  namespace: default
data:
  key: a
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: config-b
  namespace: default
data:
  key: b
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "kustomization.yaml"), []byte(kustomization), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "resources.yaml"), []byte(resources), 0o600))

	result, err := Render(dir)
	require.NoError(t, err)

	docs := strings.Split(string(result), "---")
	// Should have at least 2 documents
	count := 0
	for _, doc := range docs {
		if strings.TrimSpace(doc) != "" {
			count++
		}
	}
	assert.Equal(t, 2, count)
}
