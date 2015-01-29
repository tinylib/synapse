package synapse

import (
	"sync"
)

const (

	// increasing this wastes memory in order to save time.
	// empirically, this is large enough that map lock contention
	// is not a bottleneck over net.Pipe(), and thus it is large
	// enough for virtually all real-world use.
	rootBits = 6

	rootSize = 1 << rootBits
	rootMask = rootSize - 1
)

// waiterMap is part radix-tree, part linked list.
// we're taking advantage of the fact that sequence
// numbers increase monotonically in order to do finer-grained
// locking on the state of the map. also, we can do timeout
// and error flushing on a fine-grained basis as well.
type waiterMap [rootSize]prefixNode

// returns a sequence number's canonical node
func (w *waiterMap) node(seq uint64) *prefixNode {
	return &w[seq&rootMask]
}

// a node is just a linked list of *waiter with
// a mutex protecting access
type prefixNode struct {
	sync.Mutex
	list *waiter // stack of *waiter
}

func (n *prefixNode) pop(seq uint64) *waiter {
	n.Lock()

	// this **waiter is the pointer
	// to the previous pointer-to-waiter
	prev := &n.list

	for cur := n.list; cur != nil; cur = cur.next {
		if cur.seq == seq {
			*prev = cur.next
			cur.next = nil
			n.Unlock()
			return cur
		}
		prev = &cur.next
	}

	n.Unlock()
	return nil
}

func (n *prefixNode) insert(q *waiter) {
	n.Lock()
	n.list, q.next = q, n.list
	n.Unlock()
}

func (n *prefixNode) size() (i int) {
	n.Lock()
	for w := n.list; w != nil; w = w.next {
		i++
	}
	n.Unlock()
	return
}

// flush waiters marked with 'reap',
// and mark unmarked waiters.
func (n *prefixNode) reap() {
	n.Lock()
	prev := &n.list
	for cur := n.list; cur != nil; cur = cur.next {
		if cur.reap {
			*prev = cur.next
			cur.err = ErrTimeout
			cur.next = nil
			cur.done.Unlock()
		} else {
			cur.reap = true
			prev = &cur.next
		}
	}
	n.Unlock()
}

// flush the entire contents of the node
func (n *prefixNode) flush(err error) {
	n.Lock()
	var next *waiter
	for l := n.list; l != nil; {
		next, l.next = l.next, nil
		l.err = err
		l.done.Unlock()
		l = next
	}
	n.list = nil
	n.Unlock()
}

// insert inserts a value, and has undefined
// behavior if it already exists
func (w *waiterMap) insert(q *waiter) {
	w.node(q.seq).insert(q)
}

// remove gets a value and removes it if it exists.
// it returns nil if it didn't exist in the first place.
func (w *waiterMap) remove(seq uint64) *waiter {
	return w.node(seq).pop(seq)
}

// return the total size of the map
func (w *waiterMap) length() (count int) {
	for i := range w {
		count += w[i].size()
	}
	return count
}

// unlock every waiter with the provided error,
// and then zero out the entire contents of the map
func (w *waiterMap) flush(err error) {
	for i := range w {
		w[i].flush(err)
	}
}

// reap carries out a timeout reap.
// if (*waiter).reap==true, then delete it,
// otherwise set (*waiter).reap to true.
func (w *waiterMap) reap() {
	for i := range w {
		w[i].reap()
	}
}
