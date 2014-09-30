package synapse

import (
	"github.com/philhofer/msgp/enc"
	"sync"
)

var (
	// pool of connWrappers
	cwPool sync.Pool

	// pool of waiters
	wtPool sync.Pool
)

func init() {
	cwPool.New = func() interface{} {
		cw := &connWrapper{}
		cw.en = enc.NewEncoder(&cw.out)
		cw.dc = enc.NewDecoder(nil)
		return cw
	}

	wtPool.New = func() interface{} {
		wt := &waiter{}
		wt.en = enc.NewEncoder(&wt.buf)
		wt.dc = enc.NewDecoder(nil)
		wt.done = make(chan struct{}, 1)
		return wt
	}
}

func popWrapper(c *connHandler) *connWrapper {
	cw := cwPool.Get().(*connWrapper)
	cw.parent = c
	return cw
}

func pushWrapper(c *connWrapper) {
	if c != nil {
		cwPool.Put(c)
	}
}

func popWaiter(c *client) *waiter {
	wt := wtPool.Get().(*waiter)
	wt.parent = c
	return wt
}

func pushWaiter(w *waiter) {
	if w != nil {
		w.parent = nil
		w.buf.Reset()
		wtPool.Put(w)
	}
}
