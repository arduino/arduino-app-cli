// x is a package that provides experimental features and utilities.
package x

import "iter"

func EmptyIter[V any]() iter.Seq[V] {
	return func(yield func(V) bool) {}
}

type EnvVars map[string]string

func (e EnvVars) AsList() []string {
	list := make([]string, 0, len(e))
	for k, v := range e {
		list = append(list, k+"="+v)
	}
	return list
}
