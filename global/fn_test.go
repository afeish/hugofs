package global

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSort(t *testing.T) {
	type A1 uint64

	list := []A1{
		5, 3, 1, 2, 4,
	}

	Sort(list, func(a, b A1) int {
		return int(a - b)
	})

	assert.Equal(t, []A1{1, 2, 3, 4, 5}, list)
}

func TestOption(t *testing.T) {
	type options struct {
		cap int
	}

	withCap := func(cap int) Option[*options] {
		return OptionFunc[*options](func(opts *options) {
			opts.cap = cap
		})
	}

	defaultOpt := options{cap: 1}

	type teststruct struct {
		opt options
	}

	t1 := teststruct{
		opt: defaultOpt,
	}

	t2 := teststruct{
		opt: defaultOpt,
	}

	ApplyOptions[*options](&t1.opt, withCap(10))
	require.EqualValues(t, 10, t1.opt.cap)

	ApplyOptions[*options](&t2.opt, withCap(2))
	require.EqualValues(t, 10, t1.opt.cap)
	require.EqualValues(t, 2, t2.opt.cap)

}

func TestMax(t *testing.T) {
	a := Max[int]()
	require.EqualValues(t, 0, a)

	a = Max[int](1, 2, 4, 3)
	require.EqualValues(t, 4, a)

}
