package synapse

import (
	"github.com/tinylib/spin"
	"io"
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
	spin.Lock(&s.lock)
	if s.top != nil {
		ptr, s.top = s.top, s.top.next
		spin.Unlock(&s.lock)
		ptr.conn = w
		return
	}
	spin.Unlock(&s.lock)
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
	spin.Lock(&s.lock)
	s.top, ptr.next = ptr, s.top
	spin.Unlock(&s.lock)
	return
}

func (s *waitStack) pop(c *Client) (ptr *waiter) {
	spin.Lock(&s.lock)
	if s.top != nil {
		ptr, s.top = s.top, s.top.next
		spin.Unlock(&s.lock)
		ptr.parent = c
		return ptr
	}
	spin.Unlock(&s.lock)
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
	spin.Lock(&s.lock)
	s.top, ptr.next = ptr, s.top
	spin.Unlock(&s.lock)
	return
}
