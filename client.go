package synapse

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
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

var (
	ErrClosed = errors.New("client is closed")
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
	return newClient(conn)
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
	return newClient(conn)
}

func newClient(c net.Conn) (*client, error) {
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
		return nil, err
	}

	return cl, nil
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
	cflag   uint32             // client state; 1 is open; 0 is closed
	conn    net.Conn           // TODO: make this multiple conns
	rtt     int64              // round-trip calculation; nsec; atomic
	mlock   sync.Mutex         // to protect map access
	pending map[uint64]*waiter // map seq number to waiting handler
}

type waiter struct {
	parent *client        // parent *client
	done   chan struct{}  // for notifying response (length 1)
	buf    bytes.Buffer   // output buffer
	lead   [13]byte       // lead for seq, type, sz
	in     []byte         // response body
	en     *enc.MsgWriter // wraps buffer
	dc     *enc.MsgReader // wraps 'in' via bytes.Reader
}

func (c *client) forceClose() error {
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
	var sz uint32
	var lead [13]byte
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

		seq = binary.BigEndian.Uint64(lead[0:8])
		frame := fType(lead[8])
		sz = binary.BigEndian.Uint32(lead[9:13])
		if sz > maxFRAMESIZE {
			log.Printf("synapse client: server sent response greater than %d bytes; closing connection", maxFRAMESIZE)
			c.conn.Close()
			return
		}

		// only accept fCMD and fRES frames;
		// they are routed to waiters
		// precisely the same way
		if frame != fCMD && frame != fRES {
			// ignore
			c.conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			_, _ = io.CopyN(ioutil.Discard, bwr, int64(sz))
			c.conn.SetReadDeadline(time.Time{})
			continue
		}

		isz := int(sz)
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
			_, _ = io.CopyN(ioutil.Discard, bwr, int64(isz))
			c.conn.SetReadDeadline(time.Time{})
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

		// wakeup waiter
		w.done <- struct{}{}
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
	w.buf.Reset()
	w.buf.Write(w.lead[:])
	w.buf.WriteByte(byte(cmd))
	w.buf.Write(msg)

	// raw request body
	bts := w.buf.Bytes()
	olen := len(bts) - 13
	binary.BigEndian.PutUint64(bts[0:8], seqn)
	bts[8] = byte(fCMD)
	binary.BigEndian.PutUint32(bts[9:13], uint32(olen))
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

func (w *waiter) write(method string, in enc.MsgEncoder) error {

	// don't write if we're closing
	if atomic.LoadUint32(&w.parent.cflag) == 0 {
		return ErrClosed
	}

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

	// handle nil body
	if in != nil {
		in.EncodeTo(w.en)
	} else {
		w.en.WriteMapHeader(0)
	}

	// raw request body
	bts := w.buf.Bytes()
	olen := len(bts) - 13

	// write seq num and size to the front of the body
	binary.BigEndian.PutUint64(bts[0:8], sn)
	bts[8] = byte(fREQ)
	binary.BigEndian.PutUint32(bts[9:13], uint32(olen))

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
	if out == nil {
		_, err = w.dc.Skip()
	} else {
		_, err = out.DecodeFrom(w.dc)
	}
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

// doCommand executes a command from the client
func (c *client) sendCommand(cmd command, msg []byte) error {
	w := popWaiter(c)

	err := w.writeCommand(cmd, msg)
	if err != nil {
		return err
	}

	// wait
	<-w.done

	// bad response
	if len(w.in) == 0 {
		pushWaiter(w)
		return errors.New("no response CMD code")
	}

	if command(w.in[0]) == cmdInvalid {
		pushWaiter(w)
		return errors.New("command invalid")
	}

	act := cmdDirectory[command(w.in[0])]
	if act == nil {
		pushWaiter(w)
		return errors.New("unknown CMD code returned")
	}

	act.Client(c, c.conn, w.in[1:])
	pushWaiter(w)
	return nil
}

// perform the ping command
func (c *client) ping() error {
	return c.sendCommand(cmdPing, nil)
}

// just wrap the waitier pointer
type async struct {
	w *waiter
}

func (a async) Read(out enc.MsgDecoder) error {
	// wait
	<-a.w.done

	err := a.w.read(out)
	pushWaiter(a.w)

	// panic on a second
	// call to Read; we shouldn't
	// maintain a reference
	// to a waiter after pushWaiter()
	// under any circumstances
	a.w = nil

	return err
}

func (c *client) Async(method string, in enc.MsgEncoder) (AsyncResponse, error) {
	w := popWaiter(c)
	err := w.write(method, in)
	if err != nil {
		pushWaiter(w)
		return nil, err
	}
	return async{w}, nil
}
