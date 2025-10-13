package helpers

import (
	"iter"
)

func EmptyIter[V any]() iter.Seq[V] {
	return func(yield func(V) bool) {}
}
