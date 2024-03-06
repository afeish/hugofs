package adaptor

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/dustin/go-humanize"
)

var (
	ErrByteArrLenNotmatched = errors.New("byte array length should be equal")
)

type IntervalValue interface {
	SetStartStop(start, stop int64)
	Clone() IntervalValue
}

type BlockWriteView struct{}

func (wiv *BlockWriteView) SetStartStop(start, stop int64) {}

func (wiv *BlockWriteView) Clone() IntervalValue {
	return &BlockWriteView{}
}

type Interval[T IntervalValue] struct {
	start int64
	end   int64
	TsNs  int64
	Value T
	Prev  *Interval[T]
	Next  *Interval[T]
}

func NewInterval[T IntervalValue](start, end int64) *Interval[T] {
	return &Interval[T]{
		start: start,
		end:   end,
	}
}

func (interval *Interval[T]) Size() int64 {
	return interval.end - interval.start
}

func (interval *Interval[T]) len() int64 {
	return interval.end - interval.start
}

func (interval *Interval[T]) String() string {
	return fmt.Sprintf("[%s,%s)", humanize.IBytes(uint64(interval.start)), humanize.IBytes(uint64(interval.end)))
}

func (interval *Interval[T]) Effective() bool {
	return !(interval.start > interval.end || interval.start == math.MinInt || interval.end == math.MaxInt)
}

type IntervalList[T IntervalValue] struct {
	head *Interval[T]
	tail *Interval[T]
	mu   sync.RWMutex
}

func NewIntervalList[T IntervalValue]() *IntervalList[T] {
	head := &Interval[T]{start: math.MinInt, end: math.MinInt}
	tail := &Interval[T]{start: math.MaxInt, end: math.MaxInt}
	head.Next = tail
	tail.Prev = head
	return &IntervalList[T]{
		head: head,
		tail: tail,
	}
}

func (list *IntervalList[T]) Front() *Interval[T] {
	n := list.head.Next
	if n.start == math.MaxInt {
		return nil
	}
	return n
}

func (list *IntervalList[T]) AppendInterval(interval *Interval[T]) {
	list.mu.Lock()
	defer list.mu.Unlock()

	if list.head.Next == nil {
		list.head.Next = interval
	}
	interval.Prev = list.tail.Prev
	interval.Next = list.tail
	if list.tail.Prev != nil {
		list.tail.Prev.Next = interval
	}
	list.tail.Prev = interval
}

func (list *IntervalList[T]) AddInterval(interval *Interval[T]) {
	list.mu.Lock()
	defer list.mu.Unlock()

	start, end := interval.start, interval.end
	if interval.start >= interval.end {
		return
	}

	p := list.head
	for ; p.Next != nil && p.Next.end <= start; p = p.Next {
	}

	q := list.tail
	for ; q.Prev != nil && q.Prev.start >= end; q = q.Prev {
	}

	if p.Next != nil && p.Next.start < start {
		t := &Interval[T]{
			start: p.Next.start,
			end:   start,
		}
		p.Next = t
		if p != list.head {
			t.Prev = p
		}
		t.Next = interval
		interval.Prev = t
	} else {
		p.Next = interval
		if p != list.head {
			interval.Prev = p
		}
	}

	if q.Prev != nil && end < q.Prev.end {
		t := &Interval[T]{
			start: end,
			end:   q.Prev.end,
		}
		q.Prev = t
		if q != list.tail {
			t.Next = q
		}
		interval.Next = t
		t.Prev = interval
	} else {
		q.Prev = interval
		if q != list.tail {
			interval.Next = q
		}
	}
}

func (list *IntervalList[T]) Merge() {
	list.mu.Lock()
	defer list.mu.Unlock()
	list.doMerge()
}

func (list *IntervalList[T]) doMerge() {
	prev := list.head
	p := prev.Next
	for ; p != nil && p != list.tail; p = p.Next {
		if p.start == prev.end {
			prev.end = p.end
			prev.Next = p.Next
			if p.Next != nil {
				p.Next.Prev = prev
				continue
			}
		}

		prev = p
	}
}

func (list *IntervalList[T]) Copy(dst, src []byte) (n int, err error) {
	list.mu.RLock()
	defer list.mu.RUnlock()
	list.doMerge()

	if len(dst) != len(src) {
		return 0, ErrByteArrLenNotmatched
	}
	for p := list.head.Next; p != nil && p != list.tail; p = p.Next {
		if p.end < int64(len(src)) && p.end < int64(len(dst)) {
			n += copy(dst[p.start:p.end], src[p.start:p.end])
		}
	}
	return
}

func (list *IntervalList[T]) Total() (total int64) {
	list.mu.RLock()
	defer list.mu.RUnlock()
	for p := list.head.Next; p != nil && p != list.tail; p = p.Next {
		total += p.len()
	}
	return
}

func (list *IntervalList[T]) String() string {
	list.mu.RLock()
	defer list.mu.RUnlock()
	var sb strings.Builder
	sb.WriteRune('{')
	for p := list.head.Next; p != nil && p != list.tail; p = p.Next {
		sb.WriteString(p.String())
		if p.Next != list.tail && p.Next != nil {
			sb.WriteRune(',')
		}
	}
	sb.WriteRune('}')
	return sb.String()
}
