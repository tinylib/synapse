package synapse

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/philhofer/fwd"
	"github.com/tinylib/msgp/msgp"
)

const (
	defaultTimeout = 3000 // three seconds

	// waiter "high water mark,"
	// maximum number of queued writes.
	// we start throttling requests after
	// we reach this point.
	// TODO(maybe): make this adjustable.
	waiterHWM = 32
)

type clientState byte

const (
	zeroState clientState = iota
	closedState
	openState
)

var (
	// ErrClosed is returns when a call is attempted
	// on a closed client.
	ErrClosed = errors.New("synapse: client is closed")

	// ErrTimeout is returned when a server
	// doesn't respond to a request before
	// the client's timeout scavenger can
	// free the waiting goroutine
	ErrTimeout = errors.New("synapse: the server didn't respond in time")

	// ErrTooLarge is returned when the message
	// size is larger than 65,535 bytes.
	ErrTooLarge = errors.New("synapse: message body too large")
)

// AsyncResponse is returned by
// calls to (*Client).Async
type AsyncResponse interface {
	// Read reads the response to the
	// request into the object, returning
	// any errors encountered. Read blocks
	// until a response is received. Calling
	// Read more than once will cause undefined behavior.
	// Calling Read(nil) discards the response.
	Read(out msgp.Unmarshaler) error
}

// Dial creates a new client by dialing
// the provided network and remote address.
// The provided timeout is used as the timeout
// for requests, in milliseconds.
func Dial(network string, raddr string, timeout int64) (*Client, error) {
	conn, err := net.Dial(network, raddr)
	if err != nil {
		return nil, err
	}
	return NewClient(conn, timeout)
}

// DialTLS acts identically to Dial, except that it dials the connection
// over TLS using the provided *tls.Config.
func DialTLS(network, raddr string, timeout int64, config *tls.Config) (*Client, error) {
	conn, err := tls.Dial(network, raddr, config)
	if err != nil {
		return nil, err
	}
	return NewClient(conn, timeout)
}

// NewClient creates a new client from an
// existing net.Conn. Timeout is the maximum time,
// in milliseconds, to wait for server responses
// before sending an error to the caller. NewClient
// fails with an error if it cannot ping the server
// over the connection.
func NewClient(c net.Conn, timeout int64) (*Client, error) {
	cl := &Client{
		conn:    c,
		writing: make(chan *waiter, waiterHWM),
		done:    make(chan struct{}),
		state:   openState,
	}
	go cl.readLoop()
	go cl.writeLoop()

	if timeout <= 0 {
		timeout = defaultTimeout
	}

	go cl.timeoutLoop(timeout)

	// do a ping to check
	// for sanity
	err := cl.ping()
	if err != nil {
		cl.Close()
		return nil, fmt.Errorf("synapse: ping failed: %s", err)
	}

	return cl, nil
}

// Client is a client to
// a single synapse server.
type Client struct {
	conn    net.Conn      // connection
	csn     uint64        // sequence number; atomic
	mlock   sync.RWMutex  // protects 'state', held by writing goroutines
	writing chan *waiter  // queue to write to conn; size is effectively HWM
	done    chan struct{} // closed during (*Client).Close to shut down timeoutloop
	state   clientState   // open, closed, etc.
	pending waiterMap     // map seq number to waiting handler
}

// used to transfer control
// flow to blocking goroutines
type waiter struct {
	next   *waiter    // next in linked list, or self
	parent *Client    // parent *client
	seq    uint64     // sequence number
	done   sync.Mutex // for notifying response; locked is default
	err    error      // response error on wakeup, if applicable
	in     []byte     // response body
	reap   bool       // can reap for timeout (see: (*client).timeoutLoop())
	static bool       // is part of the statically allocated array
	_      [sizeofPtr - 2]byte
}

// Close idempotently closes the
// client's connection to the server.
// Goroutines blocked waiting for
// responses from the server will be
// unblocked with an error.
func (c *Client) Close() error {
	// don't let blocking reads/writes
	// prevent another goroutine from
	// unlocking the map lock (eventually)
	c.conn.SetWriteDeadline(time.Now().Add(1 * time.Second))
	c.conn.SetReadDeadline(time.Now().Add(1 * time.Second))

	c.mlock.Lock()
	if c.state == closedState {
		c.mlock.Unlock()
		return ErrClosed
	}
	c.state = closedState
	c.pending.flush(ErrClosed)
	c.mlock.Unlock()
	close(c.done)
	close(c.writing)
	return c.conn.Close()
}

// close with error
// sets the status of every waiting
// goroutine to 'err' and unblocks it
func (c *Client) closeError(err error) {
	err = fmt.Errorf("synapse: fatal error: %s", err)

	c.mlock.Lock()
	if c.state == closedState {
		c.mlock.Unlock()
		return
	}
	c.state = closedState
	c.pending.flush(err)
	c.mlock.Unlock()
	close(c.done)
	close(c.writing)
	c.conn.Close()
}

// a handler for io.Read() and io.Write(),
// e.g.
//  if !c.do(w.Write(data)) { goto fail }
func (c *Client) do(_ int, err error) bool {
	if err != nil {
		c.closeError(err)
		return false
	}
	return true
}

// readLoop continuously polls
// the connection for server
// responses. responses are then
// filled into the appropriate
// waiter's input buffer. it returns
// on the first error returned by Read()
func (c *Client) readLoop() {
	var seq uint64
	var sz int
	var frame fType
	var lead [leadSize]byte
	bwr := fwd.NewReaderSize(c.conn, 4096)

	for {
		if !c.do(bwr.ReadFull(lead[:])) {
			return
		}

		seq, frame, sz = readFrame(lead)

		// only accept fCMD and fRES frames;
		// they are routed to waiters
		// precisely the same way
		if frame != fCMD && frame != fRES {
			// ignore
			if !c.do(bwr.Skip(sz)) {
				return
			}
			continue
		}

		// note: this lock
		// is also acquired by
		// writing goroutines, which
		// may block while holding the lock.
		//c.mlock.Lock()
		//w, ok := c.pending[seq]
		//if ok {
		//		delete(c.pending, seq)
		//}
		//c.mlock.Unlock()

		w := c.pending.remove(seq)

		//if !ok {
		if w == nil {
			if !c.do(bwr.Skip(sz)) {
				return
			}
			continue
		}

		// fill the waiters input
		// buffer and then notify
		if cap(w.in) >= sz {
			w.in = w.in[:sz]
		} else {
			w.in = make([]byte, sz)
		}

		if !c.do(bwr.ReadFull(w.in)) {
			return
		}

		// wakeup waiter w/
		// error from last
		// read call (usually nil)
		w.err = nil
		w.done.Unlock()
	}
}

func (c *Client) writeLoop() {
	bwr := fwd.NewWriterSize(c.conn, 4096)

	// the idea here is to
	// take advantage of buffering
	// small writes, but without
	// having to periodically
	// flush the buffer from
	// a separate goroutine.
	// instead, we write from
	// the queue as many times
	// as we can without blocking,
	// and then flush.
	for {
		// wait for one write
		wt, ok := <-c.writing
		if !ok {
			return
		}
		// write it
		if !c.do(bwr.Write(wt.in)) {
			return
		}
		// try to match this write
		// with other writes
	more:
		select {
		case another, ok := <-c.writing:
			if ok {
				if !c.do(bwr.Write(another.in)) {
					return
				}
				goto more
			} else {
				bwr.Flush()
				return
			}
		default:
			if !c.do(0, bwr.Flush()) {
				return
			}
		}
	}
}

// once every 'msec' milliseconds, reap
// every pending item with reap=true, and
// set all others to reap=true.
//
// NOTE(pmh): this doesn't actually guarantee
// de-queing after the given duration;
// rather, it limits max wait time to 2*msec.
func (c *Client) timeoutLoop(msec int64) {
	tick := time.Tick(time.Millisecond * time.Duration(msec))
	for {
		select {
		case <-c.done:
			return
		case <-tick:
			c.mlock.RLock()
			c.pending.reap()
			c.mlock.RUnlock()
		}
	}
}

// write a command to the connection - works
// similarly to standard write()
func (w *waiter) writeCommand(cmd command, msg []byte) error {
	cmdlen := len(msg) + 1
	if cmdlen > maxMessageSize {
		return ErrTooLarge
	}

	seqn := atomic.AddUint64(&w.parent.csn, 1)

	// write frame + message
	need := leadSize + cmdlen
	if cap(w.in) >= need {
		w.in = w.in[:need]
	} else {
		w.in = make([]byte, need)
	}
	putFrame(w.in, seqn, fCMD, cmdlen)
	w.in[leadSize] = byte(cmd)
	copy(w.in[leadSize+1:], msg)

	w.reap = false
	w.seq = seqn

	// the whole write process must
	// be atomic with respect to
	// the map write and channel send
	// in order to prevent racing on
	// (*Client).Close
	p := w.parent
	p.mlock.RLock()
	if p.state != openState {
		p.mlock.RUnlock()
		return ErrClosed
	}
	p.pending.insert(w)
	p.writing <- w
	p.mlock.RUnlock()
	return nil
}

func (w *waiter) write(method string, in msgp.Marshaler) error {
	var err error

	// save bytes up front
	if cap(w.in) < leadSize {
		w.in = make([]byte, leadSize, 256)
	} else {
		w.in = w.in[:leadSize]
	}

	// write body
	w.in = msgp.AppendString(w.in, method)
	// handle nil body
	if in != nil {
		w.in, err = in.MarshalMsg(w.in)
		if err != nil {
			return err
		}
	} else {
		w.in = msgp.AppendMapHeader(w.in, 0)
	}

	// raw request body
	olen := len(w.in) - leadSize

	if olen > maxMessageSize {
		return ErrTooLarge
	}

	sn := atomic.AddUint64(&w.parent.csn, 1)
	putFrame(w.in, sn, fREQ, olen)
	w.reap = false
	w.seq = sn

	// the whole write process must
	// be atomic with respect to
	// the map write and channel send
	// in order to prevent racing on
	// (*Client).Close
	//
	// it's possible that writes will
	// block here for a while, but this
	// is actually to our advantage: there
	// exists a high-water mark after which
	// new requests will be bottlenecked by
	// locking around map access, which means
	// that we cannot increase memory footprint
	// without bound.
	p := w.parent
	p.mlock.RLock()
	if p.state != openState {
		p.mlock.RUnlock()
		return ErrClosed
	}
	p.pending.insert(w)
	p.writing <- w
	p.mlock.RUnlock()
	return nil
}

func (w *waiter) read(out msgp.Unmarshaler) error {
	code, body, err := msgp.ReadIntBytes(w.in)
	if err != nil {
		return err
	}
	if Status(code) != okStatus {
		return Status(code)
	}
	if out != nil {
		_, err = out.UnmarshalMsg(body)
	}
	return err
}

func (w *waiter) call(method string, in msgp.Marshaler, out msgp.Unmarshaler) error {
	err := w.write(method, in)
	if err != nil {
		return err
	}
	// wait for response
	w.done.Lock()
	if w.err != nil {
		return w.err
	}
	return w.read(out)
}

// Call sends a request to the server with 'in' as the body,
// and then decodes the response into 'out'. Call is safe
// to call from multiple goroutines simultaneously.
func (c *Client) Call(method string, in msgp.Marshaler, out msgp.Unmarshaler) error {
	w := waiters.pop(c)
	err := w.call(method, in, out)
	waiters.push(w)
	return err
}

// doCommand executes a command from the client
func (c *Client) sendCommand(cmd command, msg []byte) error {
	w := waiters.pop(c)
	err := w.writeCommand(cmd, msg)
	if err != nil {
		return err
	}

	// wait
	w.done.Lock()

	if w.err != nil {
		return w.err
	}

	// bad response
	if len(w.in) == 0 {
		waiters.push(w)
		return errors.New("no response CMD code")
	}

	ret := command(w.in[0])

	if ret == cmdInvalid || ret >= _maxcommand {
		waiters.push(w)
		return errors.New("command invalid")
	}

	act := cmdDirectory[ret]
	if act == nil {
		waiters.push(w)
		return errors.New("unknown CMD code returned")
	}

	act.Client(c, w.in[1:])
	waiters.push(w)
	return nil
}

// perform the ping command;
// returns an error if the server
// didn't respond appropriately
func (c *Client) ping() error {
	return c.sendCommand(cmdPing, nil)
}

// AsyncResponse.Read implementation
func (w *waiter) Read(out msgp.Unmarshaler) error {
	w.done.Lock()
	if w.err != nil {
		return w.err
	}
	err := w.read(out)
	waiters.push(w)
	return err
}

// Async sends a request to the server, but does not
// wait for a response. The returned AsyncResponse object
// should be used to decode the response. After the first
// call to Read(), the returned AsyncResponse object becomes
// invalid. Async is safe to call from multiple goroutines
// simultaneously.
func (c *Client) Async(method string, in msgp.Marshaler) (AsyncResponse, error) {
	w := waiters.pop(c)
	err := w.write(method, in)
	if err != nil {
		waiters.push(w)
		return nil, err
	}
	return w, nil
}
