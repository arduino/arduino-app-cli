// x is a package that provides experimental features and utilities.
package x

import "iter"

func EmptyIter[V any]() iter.Seq[V] {
	return func(yield func(V) bool) {}
}
