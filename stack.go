// +build !race

package synapse

import (
	"github.com/tinylib/spin"
)

// this file is just defines a thread-safe
// stack to use as a free-list for *connWrapper
// and *waiter structures, since they are allocated
// and de-allocated frequently and predictably.
//
// we statically allocate 'arenaSize' waiters
// and connWrappers in contiguous blocks, and
// then create a LIFO out of them during
// initialization. if we run out of elements
// in the stack, we fall back to using the
// heap. wrappers are heap-allocated if
// c.next == c, and waiters are heap allocated
// if (*waiter).static == false.

const arenaSize = 512

var (
	waiters  waitStack
	wrappers connStack

	waiterSlab  [arenaSize]waiter
	wrapperSlab [arenaSize]connWrapper
)

func init() {
	// set up the pointers and lock
	// the waiter semaphores
	waiterSlab[0].done.Lock()
	waiterSlab[0].static = true
	for i := 0; i < (arenaSize - 1); i++ {
		waiterSlab[i].next = &waiterSlab[i+1]
		waiterSlab[i+1].done.Lock()
		waiterSlab[i+1].static = true
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

func (s *connStack) pop() (ptr *connWrapper) {
	spin.Lock(&s.lock)
	if s.top != nil {
		ptr, s.top = s.top, s.top.next
		spin.Unlock(&s.lock)
		return
	}
	spin.Unlock(&s.lock)
	ptr = &connWrapper{}
	ptr.next = ptr
	return
}

func (s *connStack) push(ptr *connWrapper) {
	if ptr.next == ptr {
		return
	}
	spin.Lock(&s.lock)
	s.top, ptr.next = ptr, s.top
	spin.Unlock(&s.lock)
	return
}

// the following should always hold:
//  - ptr.next = nil
//  - ptr.parent = c
//  - ptr.done is Lock()ed
func (s *waitStack) pop(c *Client) (ptr *waiter) {
	spin.Lock(&s.lock)
	if s.top != nil {
		ptr, s.top = s.top, s.top.next
		spin.Unlock(&s.lock)
		ptr.parent = c
		ptr.next = nil
		return
	}
	spin.Unlock(&s.lock)
	ptr = &waiter{}
	ptr.parent = c
	ptr.done.Lock()
	return
}

func (s *waitStack) push(ptr *waiter) {
	ptr.parent = nil
	ptr.err = nil
	if !ptr.static {
		return
	}
	spin.Lock(&s.lock)
	s.top, ptr.next = ptr, s.top
	spin.Unlock(&s.lock)
	return
}
