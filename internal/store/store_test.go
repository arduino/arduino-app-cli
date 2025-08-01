package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var s = NewStaticStore("0.1.16")

func BenchmarkNew(b *testing.B) {
	for b.Loop() {
		x, err := s.GetBrickReadmeFromID("arduino:dbstorage_sqlstore")
		require.NoError(b, err)
		require.NotEmpty(b, x)
	}
}
