package resources

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRFC1123Label(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"./test/e2e/testdata/pruning-after/": "pruning-after",
		"simple":                             "simple",
		"../foo/bar":                         "bar",
		"/absolute/path/to/dir":              "dir",
		"UPPER-case/Dir":                     "dir",
		"a/b/../c":                           "c",
	}
	for in, want := range tests {
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, want, RFC1123Label(in))
		})
	}
}
