package global

import (
	"math/rand"
	"os"
	"reflect"
	"sort"
	"time"

	"golang.org/x/exp/constraints"
)

func IfOr[T any](flag bool, x, y T) T {
	if flag {
		return x
	}

	return y
}
func IfPtrOrNil[T any](flag bool, x T) *T {
	if flag {
		return &x
	}

	return nil
}
func Max[T constraints.Ordered](args ...T) T {
	var max T
	if len(args) == 0 {
		return max
	}
	max = args[0]
	for _, arg := range args {
		if arg > max {
			max = arg
		}
	}
	return max
}
func Min[T constraints.Ordered](args ...T) T {
	var min T
	if len(args) == 0 {
		return min
	}
	min = args[0]
	for _, arg := range args {
		if arg < min {
			min = arg
		}
	}
	return min
}
func Ptr[T any](x T) *T {
	return &x
}

// Sort sorts values (in-place) with respect to the given comparator.
func Sort[V comparable](values []V, comparator Comparator[V]) {
	sort.Sort(sortable[V]{values, comparator})
}

func SortR[V comparable](values []V, comparable Comparator[V]) []V {
	Sort(values, comparable)
	return values
}

type sortable[V comparable] struct {
	values     []V
	comparator Comparator[V]
}

func (s sortable[V]) Len() int {
	return len(s.values)
}
func (s sortable[V]) Swap(i, j int) {
	s.values[i], s.values[j] = s.values[j], s.values[i]
}
func (s sortable[V]) Less(i, j int) bool {
	return s.comparator(s.values[i], s.values[j]) < 0
}

type Comparator[T any] func(a, b T) int

func EnvOrDefault(key, def string) string {
	val := os.Getenv(key)
	if val == "" {
		return def
	}
	return val
}

func Len64(p []byte) int64 {
	return int64(len(p))
}

func RandDuration(limitInMill int32) time.Duration {
	return time.Duration(rand.Int31n(limitInMill)) * time.Millisecond
}

type Option[K any] interface {
	apply(opts K)
}

// OptionFunc wraps a function and implements the Option interface.
type OptionFunc[K any] func(K)

// apply calls the wrapped function.
func (fn OptionFunc[K]) apply(opts K) {
	fn(opts)
}

// ApplyOptions applies the provided option values to the option struct.
func ApplyOptions[K any](v K, opts ...Option[K]) {
	for i := range opts {
		opts[i].apply(v)
	}
}

func IsNil(c any) bool {
	return c == nil || (reflect.ValueOf(c).Kind() == reflect.Ptr && reflect.ValueOf(c).IsNil())
}
