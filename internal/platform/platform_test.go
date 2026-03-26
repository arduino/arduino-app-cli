package platform

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

func TestGetCodeName(t *testing.T) {
	tests := []struct {
		name  string
		files fstest.MapFS
		want  string
	}{
		{
			name: "product_name exists",
			files: fstest.MapFS{
				"sys/class/dmi/id/product_name": {Data: []byte("  Foo \n")},
			},
			want: "foo",
		},
		{
			name: "product_name exists and model exists",
			files: fstest.MapFS{
				"sys/class/dmi/id/product_name":           {Data: []byte("foo \n")},
				"sys/firmware/devicetree/base/compatible": {Data: []byte("arduino,foo")},
			},
			want: "foo",
		},
		{
			name: "single compatible",
			files: fstest.MapFS{
				"sys/firmware/devicetree/base/compatible": {Data: []byte("arduino,bar")},
			},
			want: "bar",
		},
		{
			name: "multiple compatibles",
			files: fstest.MapFS{
				"sys/firmware/devicetree/base/compatible": {Data: []byte("arduino,bar\x00some,other")},
			},
			want: "bar",
		},
		{
			name:  "no files",
			files: fstest.MapFS{},
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getCodeNameInternal(tt.files)
			require.Equal(t, tt.want, got)
		})
	}
}
