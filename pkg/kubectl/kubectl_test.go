package kubectl

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

func TestParseDryRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected cmdutil.DryRunStrategy
	}{
		{"none", cmdutil.DryRunNone},
		{"", cmdutil.DryRunNone},
		{"client", cmdutil.DryRunClient},
		{"server", cmdutil.DryRunServer},
		{"unknown", cmdutil.DryRunServer},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, parseDryRun(tt.input))
		})
	}
}

func TestWriteTempManifest(t *testing.T) {
	t.Parallel()

	data := []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n")

	path, cleanup, err := writeTempManifest(data)
	require.NoError(t, err)
	defer cleanup()

	assert.FileExists(t, path)

	content, err := os.ReadFile(path) //nolint:gosec // test file path
	require.NoError(t, err)
	assert.Equal(t, data, content)
}
