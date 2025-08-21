package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var s = NewStaticStore("0.1.16")

func BenchmarkGetBrickReadmeFromID(b *testing.B) {
	for b.Loop() {
		x, err := s.GetBrickReadmeFromID("arduino:dbstorage_sqlstore")
		require.NoError(b, err)
		require.NotEmpty(b, x)
	}
}

func BenchmarkGetBrickApiDocsPathFromID(b *testing.B) {
	for b.Loop() {
		x, err := s.GetBrickApiDocPathFromID("arduino:dbstorage_sqlstore")
		require.NoError(b, err)
		require.NotEmpty(b, x)
	}
}

func BenchmarkGetBrickCodeExamplesPathFromID(b *testing.B) {
	for b.Loop() {
		x, err := s.GetBrickCodeExamplesPathFromID("arduino:weather_forecast")
		require.NoError(b, err)
		require.NotEmpty(b, x)
	}
}
