package order

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testManifests = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
---
apiVersion: v1
kind: Namespace
metadata:
  name: my-ns
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
---
apiVersion: v1
kind: Service
metadata:
  name: my-svc
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-sa
`

func extractKinds(t *testing.T, data []byte) []string {
	t.Helper()
	var kinds []string
	for doc := range strings.SplitSeq(string(data), "---\n") {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}
		for line := range strings.SplitSeq(doc, "\n") {
			if after, ok := strings.CutPrefix(line, "kind:"); ok {
				kinds = append(kinds, strings.TrimSpace(after))
				break
			}
		}
	}
	return kinds
}

func TestSort(t *testing.T) {
	t.Parallel()

	sorted, err := Sort([]byte(testManifests))
	require.NoError(t, err)

	kinds := extractKinds(t, sorted)
	assert.Equal(t, []string{"Namespace", "ServiceAccount", "ConfigMap", "Service", "Deployment"}, kinds)
}

func TestSortReverse(t *testing.T) {
	t.Parallel()

	sorted, err := SortReverse([]byte(testManifests))
	require.NoError(t, err)

	kinds := extractKinds(t, sorted)
	assert.Equal(t, []string{"Deployment", "Service", "ConfigMap", "ServiceAccount", "Namespace"}, kinds)
}

func TestSortUnknownKind(t *testing.T) {
	t.Parallel()

	input := `apiVersion: v1
kind: Namespace
metadata:
  name: test
---
apiVersion: custom/v1
kind: MyCustomResource
metadata:
  name: test
---
apiVersion: v1
kind: Secret
metadata:
  name: test
`
	sorted, err := Sort([]byte(input))
	require.NoError(t, err)

	kinds := extractKinds(t, sorted)
	// Namespace (10) < Secret (100) < MyCustomResource (125, default)
	assert.Equal(t, []string{"Namespace", "Secret", "MyCustomResource"}, kinds)
}

func TestSortEmpty(t *testing.T) {
	t.Parallel()

	sorted, err := Sort([]byte(""))
	require.NoError(t, err)
	assert.Empty(t, sorted)
}
