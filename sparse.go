package synapse

import (
	"sync"
)

func newWaiterMap() *waiterMap {
	wmp := &waiterMap{}
	for i := range wmp.root {
		wmp.root[i].children = make([]sparseNode, 0, 4)
	}
	return wmp
}

// waiterMap is a map[uint64]*waiter,
// but it is threadsafe
type waiterMap struct {
	root [256]prefixNode
}

type prefixNode struct {
	sync.Mutex
	children []sparseNode
}

type sparseNode struct {
	seq  uint64
	wait *waiter
}

// insert inserts a value, and has undefined
// behavior if it already exists
func (w *waiterMap) insert(seq uint64, q *waiter) {
	node := &w.root[uint8(seq)]
	node.Lock()
	node.children = append(node.children, sparseNode{seq: seq, wait: q})
	node.Unlock()
}

// remove gets a value and removes it if it exists.
// it returns nil if it didn't exist in the first place.
func (w *waiterMap) remove(seq uint64) *waiter {
	node := &w.root[uint8(seq)]
	node.Lock()
	l := len(node.children)
	for i := range node.children {

		// if we find what we're looking for,
		// remove the child node and return the value
		if node.children[i].seq == seq {
			q := node.children[i].wait
			node.children, node.children[i], node.children[l-1] = node.children[:l-1], node.children[l-1], sparseNode{}
			node.Unlock()
			return q
		}

	}
	node.Unlock()
	return nil
}

func (w *waiterMap) flush(err error) {
	for i := range w.root {
		n := &w.root[i]
		n.Lock()
		for j := range n.children {
			wt := n.children[j].wait
			wt.err = err
			wt.done.Unlock()
		}
		n.children = nil
		n.Unlock()
	}
}

// reap carries out a timeout reap (basically
// a timed garbage collection)
func (w *waiterMap) reap() {
	for i := range w.root {
		w.root[i].Lock()
		for j := range w.root[i].children {
			wt := w.root[i].children[j].wait
			if wt.reap {
				wt.err = ErrTimeout
				wt.done.Unlock()
			} else {
				wt.reap = true
			}
		}
		w.root[i].Unlock()
	}
}
