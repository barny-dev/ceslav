package rowheap

import (
	"container/heap"
	"github.com/barny-dev/ceslav/internal/utilities/row"
	"github.com/barny-dev/ceslav/internal/utilities/sortfunction"
)

type RowHeap struct {
	storage  []row.Row
	sortFunc sortfunction.SortFunction
}

func New(sortFunc sortfunction.SortFunction) *RowHeap {
	return &RowHeap{
		storage:  make([]row.Row, 0),
		sortFunc: sortFunc,
	}
}

func (rh *RowHeap) Push(x any) {
	rh.storage = append(rh.storage, x.(row.Row))
}

func (rh *RowHeap) Pop() any {
	n := len(rh.storage)
	x := rh.storage[n-1]
	rh.storage = rh.storage[0 : n-1]
	return x
}

func (rh *RowHeap) PushRow(row row.Row) {
	heap.Push(rh, row)
}

func (rh *RowHeap) PopRow() row.Row {
	return heap.Pop(rh).(row.Row)
}

func (rh *RowHeap) Len() int {
	return len(rh.storage)
}

func (rh *RowHeap) Less(i, j int) bool {
	a, b := rh.storage[i], rh.storage[j]
	ord := rh.sortFunc(a, b)
	return ord <= sortfunction.OrdLE
}

func (rh *RowHeap) Swap(i, j int) {
	rh.storage[i], rh.storage[j] = rh.storage[j], rh.storage[i]
}
