package synapse

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"github.com/philhofer/msgp/enc"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type Client interface {
	Call(string, enc.MsgEncoder, enc.MsgDecoder) error
	Close() error
}

func Dial(addr string) (Client, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	cl := &client{
		conn:    conn,
		pending: make(map[uint64]*waiter),
	}
	go cl.readLoop()
	return cl, nil
}

func statusErr(s Status) error { return ErrStatus{s} }

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
	default:
		return "<unknown>"
	}
}

type client struct {
	csn     uint64
	conn    net.Conn
	mlock   sync.Mutex         // protect map access
	pending map[uint64]*waiter // map seq number to waiting handler
}

type waiter struct {
	parent *client        // parent *client
	done   chan struct{}  // for notifying response
	buf    bytes.Buffer   // output buffer
	seq    uint64         // seq number
	lead   [12]byte       // lead for seq and sz
	in     []byte         // response body
	en     *enc.MsgWriter // wraps buffer
	dc     *enc.MsgReader // wraps 'in' via bytes.Reader
}

func (c *client) Close() error {
	// TODO(philhofer): make this
	// properly shut down connections
	c.conn.SetWriteDeadline(time.Now())
	c.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	time.Sleep(1 * time.Second)
	return c.conn.Close()
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
			// ???
			if err != io.EOF {
				log.Println(err)
			}
			break
		}

		seq = binary.BigEndian.Uint64(lead[0:8])
		sz = binary.BigEndian.Uint32(lead[8:12])
		isz := int(sz)

		c.mlock.Lock()
		w, ok := c.pending[seq]
		if ok {
			delete(c.pending, seq)
		}
		c.mlock.Unlock()
		if !ok {
			// ????
			continue
		}

		// fill the waiters input
		// buffer and then notify
		if cap(w.in) >= isz {
			w.in = w.in[0:isz]
		} else {
			w.in = make([]byte, isz)
		}
		_, err = io.ReadFull(bwr, w.in)
		if err != nil {
			// ????
			// we still need to send on w.done
			log.Println(err)
		}
		w.done <- struct{}{}
	}
}

func (w *waiter) call(method string, in enc.MsgEncoder, out enc.MsgDecoder) error {
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
	w.en.WriteIdent(in)

	// raw request body
	bts := w.buf.Bytes()
	olen := len(bts) - 12

	// write seq num and size to the front of the body
	binary.BigEndian.PutUint64(bts[0:8], sn)
	binary.BigEndian.PutUint32(bts[8:12], uint32(olen))

	log.Printf("client: writing request: len=%d; seq=%d", olen, sn)
	_, err := w.parent.conn.Write(bts)
	if err != nil {
		return err
	}

	// wait for response
	<-w.done
	log.Print("client: response ack")

	// now we can read
	w.dc.Reset(bytes.NewReader(w.in))
	var code int
	code, _, err = w.dc.ReadInt()
	if Status(code) != OK {
		return statusErr(Status(code))
	}
	_, err = w.dc.ReadIdent(out)
	return err
}

func (c *client) Call(method string, in enc.MsgEncoder, out enc.MsgDecoder) error {
	w := popWaiter(c)
	err := w.call(method, in, out)
	pushWaiter(w)
	return err
}
