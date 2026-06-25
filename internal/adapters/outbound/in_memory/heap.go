package in_memory

import (
	"container/heap"
	"time"
)

type heapItem struct {
	key       string
	expiresAt time.Time
	deleted   bool
	index     int
}

type expiryHeap []*heapItem

func (h expiryHeap) Len() int           { return len(h) }
func (h expiryHeap) Less(i, j int) bool { return h[i].expiresAt.Before(h[j].expiresAt) }
func (h expiryHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *expiryHeap) Push(x any) {
	item := x.(*heapItem)
	item.index = len(*h)
	*h = append(*h, item)
}

func (h *expiryHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	item.index = -1
	return item
}

func (h *expiryHeap) peek() *heapItem {
	if len(*h) == 0 {
		return nil
	}
	return (*h)[0]
}

func newExpiryHeap() *expiryHeap {
	h := &expiryHeap{}
	heap.Init(h)
	return h
}
