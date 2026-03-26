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
			want: "Foo",
		},
		{
			name: "product_name exists and model exists",
			files: fstest.MapFS{
				"sys/class/dmi/id/product_name":      {Data: []byte("  Foo \n")},
				"sys/firmware/devicetree/base/model": {Data: []byte("Arduino SA,Bar")},
			},
			want: "Foo",
		},
		{
			name: "model with comma",
			files: fstest.MapFS{
				"sys/firmware/devicetree/base/model": {Data: []byte("Arduino SA,Bar")},
			},
			want: "Bar",
		},
		{
			name: "model with space",
			files: fstest.MapFS{
				"sys/firmware/devicetree/base/model": {Data: []byte("Arduino Foo")},
			},
			want: "Foo",
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
