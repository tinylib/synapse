package synapse

import (
	"io"
	"sync"

	"github.com/philhofer/msgp/msgp"
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
		cw.en = msgp.NewWriter(&cw.out)
		return cw
	}

	wtPool.New = func() interface{} {
		wt := &waiter{}
		wt.en = msgp.NewWriter(&wt.buf)
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
	w := wtPool.Get().(*waiter)
	w.parent = c
	w.err = nil
	return w
}

func pushWaiter(w *waiter) {
	if w != nil {
		w.parent = nil
		w.err = nil
		w.buf.Reset()
		wtPool.Put(w)
	}
}
