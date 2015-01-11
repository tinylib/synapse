package synapse

import (
	"io"
	"sync/atomic"
)

// this file is just defines a thread-safe
// stack to use as a free-list for *connWrapper
// and *waiter structures, since they are allocated
// and de-allocated frequently and predictably.

const arenaSize = 512

var (
	waiters  waitStack
	wrappers connStack

	// stack elements are persistentalloc'd
	waiterSlab  [arenaSize]waiter
	wrapperSlab [arenaSize]connWrapper
)

func init() {
	// set up the pointers and lock
	// the waiter semaphores
	waiterSlab[0].done.Lock()
	for i := 0; i < (arenaSize - 1); i++ {
		waiterSlab[i].next = &waiterSlab[i+1]
		waiterSlab[i+1].done.Lock()
		wrapperSlab[i].next = &wrapperSlab[i+1]
	}
	waiters.top = &waiterSlab[0]
	wrappers.top = &wrapperSlab[0]
}

type connStack struct {
	top  *connWrapper
	lock uint32
}

type waitStack struct {
	top  *waiter
	lock uint32
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
	ptr = &connWrapper{
		conn: w,
	}
	ptr.next = ptr
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

func (s *waitStack) pop(c *client) (ptr *waiter) {
	for !atomic.CompareAndSwapUint32(&s.lock, 0, 1) {
	}
	if s.top != nil {
		ptr, s.top = s.top, s.top.next
		atomic.StoreUint32(&s.lock, 0)
		ptr.parent = c
		return ptr
	}
	atomic.StoreUint32(&s.lock, 0)
	ptr = &waiter{
		parent: c,
	}
	ptr.next = ptr
	ptr.done.Lock()
	return
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
