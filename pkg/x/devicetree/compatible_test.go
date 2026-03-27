package devicetree

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

func TestGetCompatibleFromFS(t *testing.T) {
	tests := []struct {
		name  string
		files fstest.MapFS
		want  Compatible
	}{
		{
			name: "single compatible",
			files: fstest.MapFS{
				"sys/firmware/devicetree/base/compatible": {Data: []byte("arduino,foo")},
			},
			want: Compatible{"arduino,foo"},
		},
		{
			name: "multiple compatibles",
			files: fstest.MapFS{
				"sys/firmware/devicetree/base/compatible": {Data: []byte("arduino,bar\x00some,other")},
			},
			want: Compatible{"arduino,bar", "some,other"},
		},
		{
			name: "compatible with null bytes and whitespace",
			files: fstest.MapFS{
				"sys/firmware/devicetree/base/compatible": {Data: []byte("\x00\tarduino,baz\n\x00\x00\x00")},
			},
			want: Compatible{"arduino,baz"},
		},
		{
			name:  "no files",
			files: fstest.MapFS{},
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getCompatibleFromFS(tt.files)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestIsCompatibleWith(t *testing.T) {
	tests := []struct {
		name       string
		compatible Compatible
		prefix     string
		want       bool
	}{
		{
			name:       "matching prefix",
			compatible: Compatible{"arduino,foo"},
			prefix:     "arduino",
			want:       true,
		},
		{
			name:       "non-matching prefix",
			compatible: Compatible{"arduino,foo"},
			prefix:     "other",
			want:       false,
		},
		{
			name:       "matching full string",
			compatible: Compatible{"arduino,foo"},
			prefix:     "arduino,foo",
			want:       true,
		},
		{
			name:       "multiple compatibles with one matching",
			compatible: Compatible{"some,other", "arduino,bar"},
			prefix:     "arduino",
			want:       true,
		},
		{
			name:       "empty compatibles",
			compatible: Compatible{},
			prefix:     "arduino",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.compatible.IsCompatibleWith(tt.prefix)
			require.Equal(t, tt.want, got)
		})
	}
}
