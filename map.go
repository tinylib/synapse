package synapse

import (
	"sync"

	"github.com/tinylib/synapse/sema"
)

const (
	// number of significant bits in
	// bucket addressing. more bits
	// means more memory and better
	// worst-case performance.
	bucketBits = 6

	// number of buckets per map
	mbuckets = 1 << bucketBits

	// bitmask for bucket index
	bucketMask = mbuckets - 1
)

// wMap is really buckets of queues.
// locking is done on the bucket level, so
// concurrent reads and writes generally do
// not contend with one another. the fact that
// waiters are inserted and deleted in approximately
// FIFO order means that we can often expect best-case
// performance for removal.
type wMap [mbuckets]mNode

// returns a sequence number's canonical node
func (w *wMap) node(seq uint64) *mNode { return &w[seq&bucketMask] }

// a node is just a linked list of *waiter with
// a mutex protecting access
type mNode struct {
	sync.Mutex
	list *waiter // stack of *waiter
	tail *waiter // insert point
}

// pop searches the list from top to bottom
// for the sequence number 'seq' and removes
// the *waiter from the list if it exists. returns
// nil otherwise.
func (n *mNode) pop(seq uint64) *waiter {
	fwd := &n.list
	var prev *waiter
	for cur := n.list; cur != nil; cur = cur.next {
		if cur.seq == seq {
			if cur == n.tail {
				n.tail = prev
			}
			*fwd, cur.next = cur.next, nil
			return cur
		}
		fwd = &cur.next
		prev = cur
	}
	return nil
}

// insert puts a waiter at the tail of the list
func (n *mNode) insert(q *waiter) {
	if n.list == nil {
		n.list = q
	} else {
		n.tail.next = q
	}
	n.tail = q
}

func (n *mNode) size() (i int) {
	for w := n.list; w != nil; w = w.next {
		i++
	}
	return
}

// flush waiters marked with 'reap',
// and mark unmarked waiters.
func (n *mNode) reap() {
	fwd := &n.list
	var prev *waiter
	for cur := n.list; cur != nil; {
		if cur.reap {
			if cur == n.tail {
				n.tail = prev
			}
			*fwd, cur.next = cur.next, nil
			cur.err = ErrTimeout
			sema.Wake(&cur.done)
			cur = *fwd
		} else {
			cur.reap = true
			prev = cur
			fwd = &cur.next
			cur = cur.next
		}
	}
}

// flush the entire contents of the node
func (n *mNode) flush(err error) {
	var next *waiter
	for l := n.list; l != nil; {
		next, l.next = l.next, nil
		l.err = err
		sema.Wake(&l.done)
		l = next
	}
	n.list = nil
	n.tail = nil
}

// insert inserts a waiter keyed on its
// sequence number. this has undefined behavior
// if the waiter is already in the map. (!!!)
func (w *wMap) insert(q *waiter) {
	n := w.node(q.seq)
	n.Lock()
	n.insert(q)
	n.Unlock()
}

// remove gets a value and removes it if it exists.
// it returns nil if it didn't exist in the first place.
func (w *wMap) remove(seq uint64) *waiter {
	n := w.node(seq)
	n.Lock()
	wt := n.pop(seq)
	n.Unlock()
	return wt
}

// return the total size of the map... sort of.
// this is only used for testing. during concurrent
// access, the returned value may not be the size
// of the map at *any* fixed point in time. it's
// an approximation.
func (w *wMap) length() (count int) {
	for i := range w {
		n := &w[i]
		n.Lock()
		count += n.size()
		n.Unlock()
	}
	return count
}

// unlock every waiter with the provided error,
// and then zero out the entire contents of the map
func (w *wMap) flush(err error) {
	for i := range w {
		n := &w[i]
		n.Lock()
		n.flush(err)
		n.Unlock()
	}
}

// reap carries out a timeout reap.
// if (*waiter).reap==true, then delete it,
// otherwise set (*waiter).reap to true.
func (w *wMap) reap() {
	for i := range w {
		n := &w[i]
		n.Lock()
		n.reap()
		n.Unlock()
	}
}
