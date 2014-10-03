package synapse

import (
	"github.com/philhofer/msgp/enc"
	"io"
	"sync"
)

var (
	// pool of connWrappers (for server)
	cwPool sync.Pool

	// pool of waiters (for client)
	wtPool sync.Pool
)

func init() {
	cwPool.New = func() interface{} {
		cw := &connWrapper{}
		cw.en = enc.NewEncoder(&cw.out)
		cw.dc = enc.NewDecoder(nil) // set later
		return cw
	}

	wtPool.New = func() interface{} {
		wt := &waiter{}
		wt.en = enc.NewEncoder(&wt.buf)
		wt.dc = enc.NewDecoder(nil) // set later
		wt.done = make(chan struct{}, 1)
		return wt
	}
}

func popWrapper(w io.Writer) *connWrapper {
	cw := cwPool.Get().(*connWrapper)
	cw.conn = w
	return cw
}

func pushWrapper(c *connWrapper) {
	if c != nil {
		c.conn = nil
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
