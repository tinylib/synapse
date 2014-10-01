package synapse

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"github.com/philhofer/msgp/enc"
	"io"
	"io/ioutil"
	"log"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

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
	return newClient(conn), nil
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
	return newClient(conn), nil
}

func newClient(c net.Conn) *client {
	cl := &client{
		conn:    c,
		pending: make(map[uint64]*waiter),
	}
	go cl.readLoop()
	return cl
}

// IsNotFound returns whether or not
// the error is a synapse.NotFound error
func IsNotFound(err error) bool {
	return isStatus(err, NotFound)
}

// IsNotAuthed returns whether or not
// the error is a synapse.NotAuthed error
func IsNotAuthed(err error) bool {
	return isStatus(err, NotAuthed)
}

// IsServerError returns whether or not
// the error is a synapse.ServerError error
func IsServerError(err error) bool {
	return isStatus(err, ServerError)
}

// IsBadRequest returns whether or not
// the error is a synapse.IsBadRequest error
func IsBadRequest(err error) bool {
	return isStatus(err, BadRequest)
}

func isStatus(err error, s Status) bool {
	es, ok := err.(ErrStatus)
	if ok && es.Status == s {
		return true
	}
	return false
}

func statusErr(s Status) error { return ErrStatus{s} }

// ErrStatus is the error type returned
// for status codes that are not "OK"
type ErrStatus struct {
	Status Status
}

func (e ErrStatus) Error() string {
	switch e.Status {
	case NotFound:
		return "Not Found"
	case NotAuthed:
		return "Not Authorized"
	case ServerError:
		return "Server Error"
	case Invalid:
		return "Invalid Status"
	case BadRequest:
		return "Bad Request"
	default:
		return "<unknown>"
	}
}

type client struct {
	csn     uint64             // sequence number; atomic
	conn    net.Conn           // TODO: make this multiple conns (pending benchmark data)
	mlock   sync.Mutex         // to protect map access
	pending map[uint64]*waiter // map seq number to waiting handler
}

type waiter struct {
	parent *client        // parent *client
	done   chan struct{}  // for notifying response (length 1)
	buf    bytes.Buffer   // output buffer
	lead   [12]byte       // lead for seq and sz
	in     []byte         // response body
	en     *enc.MsgWriter // wraps buffer
	dc     *enc.MsgReader // wraps 'in' via bytes.Reader
}

func (c *client) ForceClose() error {
	err := c.conn.Close()
	if err != nil {
		return err
	}

	// flush all waiters
	c.mlock.Lock()
	for _, val := range c.pending {
		val.done <- struct{}{}
	}
	c.mlock.Unlock()
	return nil
}

func (c *client) Close() error {
	// TODO(philhofer): make this more graceful

	c.conn.SetWriteDeadline(time.Now())
	c.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	time.Sleep(1 * time.Second)
	return c.ForceClose()
}

// readLoop continuously polls
// the connection for server
// responses. responses are then
// filled into the appropriate
// waiter's input buffer
func (c *client) readLoop() {
	var seq uint64
	var sz uint32
	var lead [12]byte
	bwr := bufio.NewReader(c.conn)
	for {
		_, err := io.ReadFull(bwr, lead[:])
		if err != nil {
			// log an error if the connection
			// wasn't simply closed
			if err != io.EOF && !strings.Contains(err.Error(), "closed") {
				log.Printf("synapse client: fatal: %s", err)
			}
			break
		}

		seq = binary.BigEndian.Uint64(lead[0:8])
		sz = binary.BigEndian.Uint32(lead[8:12])
		if sz > maxFRAMESIZE {
			log.Printf("synapse client: server sent response greater than %d bytes; closing connection", maxFRAMESIZE)
			c.conn.Close()
			return
		}

		isz := int(sz)

		c.mlock.Lock()
		w, ok := c.pending[seq]
		if ok {
			delete(c.pending, seq)
		}
		c.mlock.Unlock()
		if !ok {
			// TODO: figure out how to handle this better
			//
			// discard response...
			log.Printf("discarding response #%d; no pending waiter", seq)
			c.conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			_, _ = io.CopyN(ioutil.Discard, bwr, int64(isz))
			continue
		}

		// fill the waiters input
		// buffer and then notify
		if cap(w.in) >= isz {
			w.in = w.in[0:isz]
		} else {
			w.in = make([]byte, isz)
		}

		// don't block forever on reading the request body -
		// if we haven't already buffered the body, we set
		// a deadline for the next read
		deadline := false
		if bwr.Buffered() < isz {
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
		w.done <- struct{}{}
	}
}

func (w *waiter) write(method string, in enc.MsgEncoder) error {
	// get sequence number
	sn := atomic.AddUint64(&w.parent.csn, 1)

	// enqueue
	w.parent.mlock.Lock()
	w.parent.pending[sn] = w
	w.parent.mlock.Unlock()

	// save 12 bytes up front
	w.buf.Reset()
	w.en.Write(w.lead[:])

	// write body
	w.en.WriteString(method)
	in.EncodeTo(w.en)

	// raw request body
	bts := w.buf.Bytes()
	olen := len(bts) - 12

	// write seq num and size to the front of the body
	binary.BigEndian.PutUint64(bts[0:8], sn)
	binary.BigEndian.PutUint32(bts[8:12], uint32(olen))

	_, err := w.parent.conn.Write(bts)
	if err != nil {
		// dequeue
		w.parent.mlock.Lock()
		delete(w.parent.pending, sn)
		w.parent.mlock.Unlock()
		return err
	}
	return nil
}

func (w *waiter) read(out enc.MsgDecoder) error {
	var (
		code int
		err  error
	)
	w.dc.Reset(bytes.NewReader(w.in))
	code, _, err = w.dc.ReadInt()
	if Status(code) != OK {
		return statusErr(Status(code))
	}
	_, err = out.DecodeFrom(w.dc)
	return err
}

func (w *waiter) call(method string, in enc.MsgEncoder, out enc.MsgDecoder) error {
	err := w.write(method, in)
	if err != nil {
		return err
	}

	// wait for response
	<-w.done

	// now we can read
	return w.read(out)
}

func (c *client) Call(method string, in enc.MsgEncoder, out enc.MsgDecoder) error {
	// grab a waiter from the heap,
	// make the call, put it back
	w := popWaiter(c)
	err := w.call(method, in, out)
	pushWaiter(w)
	return err
}

// AsyncHandler is returned by
// calls to client.Async
type AsyncHandler interface {
	// Read reads the response to the
	// request into the decoder, returning
	// any errors encountered. Read blocks
	// until a response is received. Calling
	// Read more than once will cause a panic.
	Read(out enc.MsgDecoder) error
}

// just wrap the waitier
// pointer
type async struct {
	w *waiter
}

func (a async) Read(out enc.MsgDecoder) error {
	// wait
	<-a.w.done

	err := a.w.read(out)
	pushWaiter(a.w)
	a.w = nil
	return err
}

func (c *client) Async(method string, in enc.MsgEncoder) (AsyncHandler, error) {
	w := popWaiter(c)
	err := w.write(method, in)
	if err != nil {
		return nil, err
	}
	return async{w}, nil
}
