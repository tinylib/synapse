package synapse

import (
	"io"
	"sync/atomic"
)

// this file is just defines a thread-safe
// stack to use as a free-list for *connWrapper
// and *waiter structures, since they are allocated
// and de-allocated frequently and predictably.

const arenaSize = 256

var (
	waiters  = newWaitStack(arenaSize)
	wrappers = newConnStack(arenaSize)
)

type connStack struct {
	top  *connWrapper
	lock uint32
}

type waitStack struct {
	top  *waiter
	lock uint32
}

func newConnStack(size int) *connStack {
	slab := make([]connWrapper, size)
	for i := 0; i < (size - 1); i++ {
		slab[i].next = &slab[i+1]
	}
	return &connStack{top: &slab[0]}
}

func (s *connStack) pop(w io.Writer) (ptr *connWrapper) {
	for !atomic.CompareAndSwapUint32(&s.lock, 0, 1) {
	}
	if s.top != nil {
		ptr, s.top = s.top, s.top.next
		atomic.StoreUint32(&s.lock, 0)
		ptr.conn = w
		return
	}
	atomic.StoreUint32(&s.lock, 0)
	ptr = &connWrapper{}
	ptr.next = ptr
	ptr.conn = w
	return
}

func (s *connStack) push(ptr *connWrapper) {
	ptr.conn = nil
	if ptr.next == ptr {
		return
	}
	for !atomic.CompareAndSwapUint32(&s.lock, 0, 1) {
	}
	s.top, ptr.next = ptr, s.top
	atomic.StoreUint32(&s.lock, 0)
	return
}

func newWaitStack(size int) *waitStack {
	slab := make([]waiter, size)
	slab[0].done.Lock()
	for i := 0; i < (size - 1); i++ {
		slab[i].next = &slab[i+1]
		slab[i+1].done.Lock()
	}
	return &waitStack{top: &slab[0]}
}

func (s *waitStack) pop(c *client) (ptr *waiter) {
	for !atomic.CompareAndSwapUint32(&s.lock, 0, 1) {
	}
	if s.top != nil {
		ptr, s.top = s.top, s.top.next
		atomic.StoreUint32(&s.lock, 0)
		ptr.parent = c
		return ptr
	}
	ptr = &waiter{}
	ptr.done.Lock()
	ptr.parent = c
	return ptr
}

func (s *waitStack) push(ptr *waiter) {
	ptr.parent = nil
	ptr.err = nil
	if ptr.next == ptr {
		return
	}
	for !atomic.CompareAndSwapUint32(&s.lock, 0, 1) {
	}
	s.top, ptr.next = ptr, s.top
	atomic.StoreUint32(&s.lock, 0)
	return
}
