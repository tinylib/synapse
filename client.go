package synapse

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"github.com/philhofer/msgp/msgp"
	"io"
	"io/ioutil"
	"log"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	// ErrClosed is returns when a call is attempted
	// on a closed client
	ErrClosed = errors.New("client is closed")

	// ErrTimeout is returned when a server
	// doesn't respond to a request before
	// the client's timeout scavenger can
	// free the waiting goroutine
	ErrTimeout = errors.New("the server didn't respond in time")

	// ErrTooLarge is returned when the message
	// size is larger than 65,535 bytes.
	ErrTooLarge = errors.New("message body too large")
)

// Client is the interface fulfilled
// by synapse clients.
type Client interface {
	// Call asks the server to perform 'method' on 'in' and
	// return the response to 'out'.
	Call(method string, in msgp.Marshaler, out msgp.Unmarshaler) error

	// Async writes the request to the connection
	// and returns a handler that can be used
	// to wait for the response.
	Async(method string, in msgp.Marshaler) (AsyncResponse, error)

	// Close closes the client.
	Close() error
}

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

// DialTCP creates a new client to the server
// located at the provided address.
func DialTCP(address string) (Client, error) {
	addr, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		return nil, err
	}
	conn.SetKeepAlive(true)
	return newClient(conn, 1000)
}

// DialUnix creates a new client to the
// server listening on the provided
// unix socket address
func DialUnix(address string) (Client, error) {
	addr, err := net.ResolveUnixAddr("unix", address)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialUnix("unix", nil, addr)
	if err != nil {
		return nil, err
	}
	return newClient(conn, 200)
}

// DialUDP creates a new client that
// operates over UDP
func DialUDP(address string) (Client, error) {
	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, err
	}
	return newClient(conn, 1000)
}

// NewClient creates a new client from an
// existing net.Conn. Timeout is the maximum time,
// in milliseconds, to wait for server responses
// before sending an error to the caller.
func NewClient(c net.Conn, timeout int64) (Client, error) {
	return newClient(c, timeout)
}

func newClient(c net.Conn, timeout int64) (*client, error) {
	cl := &client{
		cflag:   1,
		conn:    c,
		pending: make(map[uint64]*waiter),
	}
	go cl.readLoop()

	// do a ping to check
	// for sanity
	err := cl.ping()
	if err != nil {
		cl.conn.Close()
		return nil, fmt.Errorf("synapse client: attempt to ping the server failed: %s", err)
	}

	go cl.timeoutLoop(timeout)

	return cl, nil
}

type client struct {
	csn     uint64             // sequence number; atomic
	cflag   uint32             // client state; 1 is open; 0 is closed
	conn    net.Conn           // TODO: make this multiple conns
	rtt     int64              // round-trip calculation; nsec; atomic
	mlock   sync.Mutex         // to protect map access
	pending map[uint64]*waiter // map seq number to waiting handler
}

type waiter struct {
	next   *waiter        // next in linked list, or self
	parent *client        // parent *client
	done   sync.Mutex     // for notifying response; locked is default
	err    error          // response error on wakeup, if applicable
	etime  int64          // enqueue time, for timeout
	in     []byte         // response body
	lead   [leadSize]byte // lead for seq, type, sz
}

func (c *client) forceClose() error {
	err := c.conn.Close()
	if err != nil {
		return err
	}

	// flush all waiters
	c.mlock.Lock()
	for _, val := range c.pending {
		val.err = ErrClosed
		val.done.Unlock()
	}
	c.mlock.Unlock()
	return nil
}

func (c *client) Close() error {
	// already stopped
	if !atomic.CompareAndSwapUint32(&c.cflag, 1, 0) {
		return nil
	}

	// if we don't have receivers
	// pending; then we're safe
	// to close immediately
	c.mlock.Lock()
	pending := len(c.pending)
	c.mlock.Unlock()
	if pending == 0 {
		return c.forceClose()
	}

	// we have pending conns; deadline them
	c.conn.SetWriteDeadline(time.Now())
	c.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	time.Sleep(1 * time.Second)
	return c.forceClose()
}

// readLoop continuously polls
// the connection for server
// responses. responses are then
// filled into the appropriate
// waiter's input buffer
func (c *client) readLoop() {
	var seq uint64
	var sz int
	var frame fType
	var lead [leadSize]byte
	bwr := bufio.NewReader(c.conn)

	for {
		_, err := io.ReadFull(bwr, lead[:])
		if err != nil {
			if atomic.LoadUint32(&c.cflag) == 0 {
				// we received a FIN flag or
				// we intended to close
				return
			}

			// log an error if the connection
			// wasn't simply closed
			if err != io.EOF && !strings.Contains(err.Error(), "closed") {
				log.Printf("synapse client: fatal: %s", err)
			}
			break
		}

		seq, frame, sz = readFrame(lead)

		// only accept fCMD and fRES frames;
		// they are routed to waiters
		// precisely the same way
		if frame != fCMD && frame != fRES {
			// ignore
			_, _ = io.CopyN(ioutil.Discard, bwr, int64(sz))
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
			_, _ = io.CopyN(ioutil.Discard, bwr, int64(sz))
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
		_, err = io.ReadFull(bwr, w.in)
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

// once every 'msec' milliseconds, check
// that no waiter has been waiting for longer
// than 'msec'. if so, dequeue it with an error
func (c *client) timeoutLoop(msec int64) {
	for {
		time.Sleep(time.Millisecond * time.Duration(msec))
		if atomic.LoadUint32(&c.cflag) == 0 {
			return
		}
		now := time.Now().Unix()
		c.mlock.Lock()
		for seq, w := range c.pending {
			if now-w.etime > msec {
				delete(c.pending, seq)
				w.err = ErrTimeout
				w.done.Unlock()
			}
		}
		c.mlock.Unlock()
	}
}

// write a command to the connection - works
// similarly to standard write()
func (w *waiter) writeCommand(cmd command, msg []byte) error {
	if atomic.LoadUint32(&w.parent.cflag) == 0 {
		return ErrClosed
	}

	// queue
	seqn := atomic.AddUint64(&w.parent.csn, 1)
	w.parent.mlock.Lock()
	w.parent.pending[seqn] = w
	w.parent.mlock.Unlock()

	// write buffered msg
	var buf bytes.Buffer
	buf.Write(w.lead[:])
	buf.WriteByte(byte(cmd))
	buf.Write(msg)

	// raw request body
	bts := buf.Bytes()
	olen := len(bts) - leadSize
	if olen > maxMessageSize {
		// dequeue
		w.parent.mlock.Lock()
		delete(w.parent.pending, seqn)
		w.parent.mlock.Unlock()
		return errors.New("message too large")
	}

	putFrame(bts, seqn, fCMD, olen)
	w.etime = time.Now().Unix()
	_, err := w.parent.conn.Write(bts)
	if err != nil {
		// dequeue
		w.parent.mlock.Lock()
		delete(w.parent.pending, seqn)
		w.parent.mlock.Unlock()
		return err
	}
	return nil
}

func (w *waiter) write(method string, in msgp.Marshaler) error {

	// don't write if we're closing
	if atomic.LoadUint32(&w.parent.cflag) == 0 {
		return ErrClosed
	}

	// get sequence number
	sn := atomic.AddUint64(&w.parent.csn, 1)

	// record enqueue time
	w.etime = time.Now().Unix()

	// enqueue
	w.parent.mlock.Lock()
	w.parent.pending[sn] = w
	w.parent.mlock.Unlock()

	var err error

	// save 12 bytes up front
	w.in = append(w.in[0:0], w.lead[:]...)

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
		return errors.New("message too large")
	}

	putFrame(w.in, sn, fREQ, olen)

	_, err = w.parent.conn.Write(w.in)
	if err != nil {
		// dequeue
		w.parent.mlock.Lock()
		delete(w.parent.pending, sn)
		w.parent.mlock.Unlock()
		return err
	}
	return nil
}

func (w *waiter) read(out msgp.Unmarshaler) error {
	var (
		code int
		err  error
		body []byte
	)
	code, body, err = msgp.ReadIntBytes(w.in)
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

func (c *client) Call(method string, in msgp.Marshaler, out msgp.Unmarshaler) error {
	// grab a waiter from the heap,
	// make the call, put it back
	w := waiters.pop(c)
	err := w.call(method, in, out)
	waiters.push(w)
	return err
}

// doCommand executes a command from the client
func (c *client) sendCommand(cmd command, msg []byte) error {
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
func (c *client) ping() error {
	return c.sendCommand(cmdPing, nil)
}

// AsyncResponse.Read
func (w *waiter) Read(out msgp.Unmarshaler) error {
	w.done.Lock()
	if w.err != nil {
		return w.err
	}
	err := w.read(out)
	waiters.push(w)
	return err
}

func (c *client) Async(method string, in msgp.Marshaler) (AsyncResponse, error) {
	w := waiters.pop(c)
	err := w.write(method, in)
	if err != nil {
		waiters.push(w)
		return nil, err
	}
	return w, nil
}
