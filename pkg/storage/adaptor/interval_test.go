package adaptor

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

type intVal struct{}

func (iv *intVal) SetStartStop(start, stop int64) {}

func (iv *intVal) Clone() IntervalValue {
	return &intVal{}
}

func newIntInterval(start, end int) *Interval[*intVal] {
	return &Interval[*intVal]{
		start: int64(start),
		end:   int64(end),
	}
}

func TestIntervalLen(t *testing.T) {
	list := NewIntervalList[*intVal]()

	list.AppendInterval(newIntInterval(0, 2))
	list.AppendInterval(newIntInterval(2, 3))
	list.AppendInterval(newIntInterval(5, 10))
	require.EqualValues(t, "{[0 B,2 B),[2 B,3 B),[5 B,10 B)}", list.String())
	require.EqualValues(t, 8, list.Total())
}

func TestAppendInterval(t *testing.T) {
	list := NewIntervalList[*intVal]()
	list.AppendInterval(newIntInterval(0, 100))
	list.AppendInterval(newIntInterval(50, 150))
	list.AppendInterval(newIntInterval(200, 250))
	list.AppendInterval(newIntInterval(225, 250))
	list.AppendInterval(newIntInterval(175, 210))
	list.AppendInterval(newIntInterval(0, 25))

	fmt.Printf("interval: %s", list.String())
}

func TestAddInterval(t *testing.T) {
	list := NewIntervalList[*intVal]()
	list.AddInterval(newIntInterval(0, 100))
	list.AddInterval(newIntInterval(50, 150))
	list.AddInterval(newIntInterval(200, 250))
	list.AddInterval(newIntInterval(225, 250))
	list.AddInterval(newIntInterval(175, 210))
	list.AddInterval(newIntInterval(0, 25))
	require.EqualValues(t, "{[0 B,25 B),[25 B,50 B),[50 B,150 B),[175 B,210 B),[210 B,225 B),[225 B,250 B)}", list.String())
	require.EqualValues(t, 225, list.Total())

	list.Merge()
	require.EqualValues(t, "{[0 B,150 B),[175 B,250 B)}", list.String())
}

func TestAddInterval2(t *testing.T) {
	list := NewIntervalList[*intVal]()
	list.AddInterval(newIntInterval(50, 100))
	list.AddInterval(newIntInterval(0, 100))

	require.EqualValues(t, "{[0 B,100 B)}", list.String())
	require.EqualValues(t, 100, list.Total())

	list.Merge()
	require.EqualValues(t, "{[0 B,100 B)}", list.String())
}

func TestAddInterval4(t *testing.T) {
	list := NewIntervalList[*intVal]()
	list.AddInterval(newIntInterval(50, 100))
	list.AddInterval(newIntInterval(50, 100))
	list.AddInterval(newIntInterval(50, 90))

	require.EqualValues(t, "{[50 B,90 B),[90 B,100 B)}", list.String())
	require.EqualValues(t, 50, list.Total())
	list.Merge()
	require.EqualValues(t, "{[50 B,100 B)}", list.String())
}

func TestAddInterval5(t *testing.T) {
	list := NewIntervalList[*intVal]()
	list.AddInterval(newIntInterval(50, 100))
	list.AddInterval(newIntInterval(60, 90))

	require.EqualValues(t, "{[50 B,60 B),[60 B,90 B),[90 B,100 B)}", list.String())
	require.EqualValues(t, 50, list.Total())

	list.Merge()
	require.EqualValues(t, "{[50 B,100 B)}", list.String())
}

func TestAddInterval6(t *testing.T) {
	list := NewIntervalList[*intVal]()
	list.AddInterval(newIntInterval(50, 100))
	list.AddInterval(newIntInterval(60, 110))

	require.EqualValues(t, "{[50 B,60 B),[60 B,110 B)}", list.String())
	require.EqualValues(t, 60, list.Total())

	list.Merge()
	require.EqualValues(t, "{[50 B,110 B)}", list.String())
}

func TestAddInterval7(t *testing.T) {
	list := NewIntervalList[*intVal]()
	list.AddInterval(newIntInterval(25, 100))
	list.AddInterval(newIntInterval(50, 70))
	list.AddInterval(newIntInterval(90, 150))
	list.AddInterval(newIntInterval(180, 200))

	require.EqualValues(t, "{[25 B,50 B),[50 B,70 B),[70 B,90 B),[90 B,150 B),[180 B,200 B)}", list.String())
	require.EqualValues(t, 145, list.Total())

	list.Merge()
	require.EqualValues(t, "{[25 B,150 B),[180 B,200 B)}", list.String())
}
