package synapse

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/philhofer/fwd"
	"github.com/tinylib/msgp/msgp"
)

const (
	defaultTimeout = 3000 // three seconds
)

var (
	// ErrClosed is returns when a call is attempted
	// on a closed client
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
// calls to client.Async
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
func Dial(network string, laddr string, timeout int64) (*Client, error) {
	conn, err := net.Dial(network, laddr)
	if err != nil {
		return nil, err
	}
	return NewClient(conn, timeout)
}

// DialTLS acts identically to Dial, except that it dials the connection
// over TLS using the provided *tls.Config.
func DialTLS(network, laddr string, timeout int64, config *tls.Config) (*Client, error) {
	conn, err := tls.Dial(network, laddr, config)
	if err != nil {
		return nil, err
	}
	return NewClient(conn, timeout)
}

// NewClient creates a new client from an
// existing net.Conn. Timeout is the maximum time,
// in milliseconds, to wait for server responses
// before sending an error to the caller.
func NewClient(c net.Conn, timeout int64) (*Client, error) {
	cl := &Client{
		cflag:   1,
		conn:    c,
		pending: make(map[uint64]*waiter),
		writing: make(chan *waiter, 32),
		done:    make(chan struct{}),
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
		cl.conn.Close()
		return nil, fmt.Errorf("synapse client: attempt to ping the server failed: %s", err)
	}

	return cl, nil
}

// Client is a client to
// a single synapse server.
type Client struct {
	conn    net.Conn           // TODO(maybe): make this multiple conns
	csn     uint64             // sequence number; atomic
	rtt     int64              // round-trip calculation; nsec; atomic
	mlock   sync.Mutex         // to protect map access
	pending map[uint64]*waiter // map seq number to waiting handler
	writing chan *waiter       // queue to write to conn
	cflag   uint32             // client state; 1 is open; 0 is closed
	done    chan struct{}      // for shutting down timeout loop, etc.
}

// used to transfer control
// flow to blocking goroutines
type waiter struct {
	next   *waiter    // next in linked list, or self
	parent *Client    // parent *client
	done   sync.Mutex // for notifying response; locked is default
	err    error      // response error on wakeup, if applicable
	in     []byte     // response body
	reap   bool       // can reap for timeout (see: (*client).timeoutLoop())
}

// you must hold c.mlock to call forceClose
func (c *Client) forceClose() error {
	err := c.conn.Close()
	for _, val := range c.pending {
		val.err = ErrClosed
		val.done.Unlock()
	}
	close(c.done)
	c.mlock.Unlock()
	return err
}

// Close idempotently closes the
// client's connection to the server.
// Goroutines blocked waiting for
// responses from the server will be
// unblocked with an error after 1 second.
func (c *Client) Close() error {
	// already stopped
	if !atomic.CompareAndSwapUint32(&c.cflag, 1, 0) {
		return ErrClosed
	}
	// if we don't have receivers
	// pending; then we're safe
	// to close immediately
	c.mlock.Lock()
	if len(c.pending) == 0 {
		return c.forceClose()
	}
	c.mlock.Unlock()

	// we have pending conns; deadline them
	c.conn.SetWriteDeadline(time.Now().Add(1 * time.Second))
	c.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	time.Sleep(1 * time.Second)
	c.mlock.Lock()
	return c.forceClose()
}

// readLoop continuously polls
// the connection for server
// responses. responses are then
// filled into the appropriate
// waiter's input buffer
func (c *Client) readLoop() {
	var seq uint64
	var sz int
	var frame fType
	var lead [leadSize]byte
	bwr := fwd.NewReaderSize(c.conn, 4096)

	for {
		_, err := bwr.ReadFull(lead[:])
		if err != nil {
			if atomic.LoadUint32(&c.cflag) == 0 {
				// we intended to close
				return
			}

			// log an error if the connection
			// wasn't simply closed
			if err != io.EOF && err != io.ErrUnexpectedEOF && !strings.Contains(err.Error(), "closed") {
				log.Printf("synapse client: fatal: %s", err)
			}
			return
		}

		seq, frame, sz = readFrame(lead)

		// only accept fCMD and fRES frames;
		// they are routed to waiters
		// precisely the same way
		if frame != fCMD && frame != fRES {
			// ignore
			_, err := bwr.Skip(sz)
			if err != nil {
				log.Printf("synapse client: fatal: %s", err)
				return
			}
			continue
		}

		c.mlock.Lock()
		w, ok := c.pending[seq]
		if ok {
			delete(c.pending, seq)
		}
		c.mlock.Unlock()
		if !ok {
			// discard response...
			log.Printf("synapse client: discarding response #%d; no pending waiter", seq)
			c.conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			bwr.Skip(sz)
			c.conn.SetReadDeadline(time.Time{})
			continue
		}

		// fill the waiters input
		// buffer and then notify
		if cap(w.in) >= sz {
			w.in = w.in[0:sz]
		} else {
			w.in = make([]byte, sz)
		}

		// don't block forever on reading the request body -
		// if we haven't already buffered the body, we set
		// a deadline for the next read
		deadline := false
		if bwr.Buffered() < sz {
			deadline = true
			c.conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		}
		_, err = bwr.ReadFull(w.in)
		if err != nil {
			// TODO: better info here
			// we still need to send on w.done
			log.Printf("synapse client: error on read: %s", err)
		}
		if deadline {
			// clear deadline
			c.conn.SetReadDeadline(time.Time{})
		}

		// wakeup waiter w/
		// error from last
		// read call (usually nil)
		w.err = err
		w.done.Unlock()
	}
}

func (c *Client) writeLoop() {
	bwr := fwd.NewWriterSize(c.conn, 4096)

	for {
		// wait for one write
		wt, ok := <-c.writing
		if !ok {
			return
		}
		// write it
		_, err := bwr.Write(wt.in)
		if err != nil {
			c.neterr(err)
			return
		}
		// try to match this write
		// with other writes
	more:
		select {
		case another, ok := <-c.writing:
			if ok {
				_, err = bwr.Write(another.in)
				if err != nil {
					c.neterr(err)
					return
				}
				goto more
			}
		default:
		}
		err = bwr.Flush()
		if err != nil {
			c.neterr(err)
			return
		}
	}
}

func (c *Client) neterr(err error) {
	if atomic.LoadUint32(&c.cflag) != 0 {
		log.Println("synapse client: fatal:", err)
		c.Close()
	}
}

// once every 'msec' milliseconds, reap
// every pending item with reap=true, and
// set all others to reap=true.
func (c *Client) timeoutLoop(msec int64) {
	for {
		select {
		case <-c.done:
			return
		case <-time.After(time.Millisecond * time.Duration(msec)):
			c.mlock.Lock()
			for seq, w := range c.pending {
				if w.reap {
					delete(c.pending, seq)
					w.err = ErrTimeout
					w.done.Unlock()
				} else {
					w.reap = true
				}
			}
			c.mlock.Unlock()
		}
	}
}

// write a command to the connection - works
// similarly to standard write()
func (w *waiter) writeCommand(cmd command, msg []byte) error {
	if atomic.LoadUint32(&w.parent.cflag) == 0 {
		return ErrClosed
	}
	cmdlen := len(msg) + 1
	if cmdlen > maxMessageSize {
		return ErrTooLarge
	}

	// get in pending list
	seqn := atomic.AddUint64(&w.parent.csn, 1)
	w.reap = false
	w.parent.mlock.Lock()
	w.parent.pending[seqn] = w
	w.parent.mlock.Unlock()

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

	w.parent.writing <- w
	return nil
}

func (w *waiter) write(method string, in msgp.Marshaler) error {

	// don't write if we're closing
	if atomic.LoadUint32(&w.parent.cflag) == 0 {
		return ErrClosed
	}

	// get sequence number
	sn := atomic.AddUint64(&w.parent.csn, 1)

	// enqueue
	w.reap = false
	w.parent.mlock.Lock()
	w.parent.pending[sn] = w
	w.parent.mlock.Unlock()

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
		// dequeue
		w.parent.mlock.Lock()
		delete(w.parent.pending, sn)
		w.parent.mlock.Unlock()
		return ErrTooLarge
	}

	putFrame(w.in, sn, fREQ, olen)
	w.parent.writing <- w
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
// and then decodes the response into 'out'.
func (c *Client) Call(method string, in msgp.Marshaler, out msgp.Unmarshaler) error {
	// grab a waiter from the heap,
	// make the call, put it back
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

	if command(w.in[0]) == cmdInvalid {
		waiters.push(w)
		return errors.New("command invalid")
	}

	act := cmdDirectory[command(w.in[0])]
	if act == nil {
		waiters.push(w)
		return errors.New("unknown CMD code returned")
	}

	act.Client(c, c.conn, w.in[1:])
	waiters.push(w)
	return nil
}

// perform the ping command
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
// should be used to decode the response.
func (c *Client) Async(method string, in msgp.Marshaler) (AsyncResponse, error) {
	w := waiters.pop(c)
	err := w.write(method, in)
	if err != nil {
		waiters.push(w)
		return nil, err
	}
	return w, nil
}
