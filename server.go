package synapse

import (
	"crypto/tls"
	"math"
	"net"
	"sync"
	"unsafe"

	"github.com/philhofer/fwd"
	"github.com/tinylib/msgp/msgp"
)

const (
	// used to compute pad sizes
	sizeofPtr = unsafe.Sizeof((*byte)(nil))

	// leadSize is the size of a "lead frame"
	leadSize = 11

	// maxMessageSize is the maximum size of a message
	maxMessageSize = math.MaxUint16
)

// All writes (on either side) need to be atomic; conn.Write() is called exactly once and
// should contain the entirety of the request (client-side) or response (Server-side).
//
// In principle, the client can operate on any net.Conn, and the
// Server can operate on any net.Listener.

// Serve starts a Server on 'l' that serves
// the supplied handler. It blocks until the
// listener closes.
func Serve(l net.Listener, service string, h Handler) error {
	a := l.Addr()
	s := Service{
		name: service,
		net:  a.Network(),
		addr: a.String(),
		host: hostid,
	}
	cache(&s)
	for {
		c, err := l.Accept()
		if err != nil {
			uncache(&s)
			return err
		}
		go ServeConn(c, service, h)
	}
}

// ListenAndServeTLS acts identically to ListenAndServe, except that
// it expects connections over TLS1.2 (see crypto/tls). Additionally,
// files containing a certificate and matching private key for the
// Server must be provided. If the certificate is signed by a
// certificate authority, the certFile should be the concatenation of
// the server's certificate followed by the CA's certificate.
func ListenAndServeTLS(network, laddr, service, certFile, keyFile string, h Handler) error {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return err
	}

	l, err := tls.Listen(network, laddr, &tls.Config{
		Certificates: []tls.Certificate{cert},
	})
	if err != nil {
		return err
	}
	return Serve(l, service, h)
}

// ListenAndServe opens up a network listener
// on the provided network and local address
// and begins serving with the provided handler.
// ListenAndServe blocks until there is a fatal
// listener error.
func ListenAndServe(network, laddr, service string, h Handler) error {
	l, err := net.Listen(network, laddr)
	if err != nil {
		return err
	}
	return Serve(l, service, h)
}

// ServeConn serves an individual network
// connection. It blocks until the connection
// is closed or it encounters a fatal error.
func ServeConn(c net.Conn, service string, h Handler) {
	ch := connHandler{
		svcname: service,
		conn:    c,
		h:       h,
		remote:  c.RemoteAddr(),
		writing: make(chan *connWrapper, 32),
		route:   getRoute(c),
	}
	go ch.writeLoop()
	ch.connLoop()     // returns on connection close
	ch.wg.Wait()      // wait for handlers to return
	close(ch.writing) // close output queue
}

func readFrame(lead [leadSize]byte) (seq uint64, ft fType, sz int) {
	seq = (uint64(lead[0]) << 56) | (uint64(lead[1]) << 48) |
		(uint64(lead[2]) << 40) | (uint64(lead[3]) << 32) |
		(uint64(lead[4]) << 24) | (uint64(lead[5]) << 16) |
		(uint64(lead[6]) << 8) | (uint64(lead[7]))
	ft = fType(lead[8])
	sz = int((uint16(lead[9]) << 8) | uint16(lead[10]))
	return
}

func putFrame(bts []byte, seq uint64, ft fType, sz int) {
	bts[0] = byte(seq >> 56)
	bts[1] = byte(seq >> 48)
	bts[2] = byte(seq >> 40)
	bts[3] = byte(seq >> 32)
	bts[4] = byte(seq >> 24)
	bts[5] = byte(seq >> 16)
	bts[6] = byte(seq >> 8)
	bts[7] = byte(seq)
	bts[8] = byte(ft)
	usz := uint16(sz)
	bts[9] = byte(usz >> 8)
	bts[10] = byte(usz)
}

// route is the network pathway
// assocaited with a particular
// conneciton (global, link-local,
// loopback, etc.)
type route uint16

const (
	routeUnknown   route = iota // no idea
	routeGlobal                 // globally routable
	routeLinkLocal              // link-local
	routeOSLocal                // same machine (unix socket / loopback)
)

func getRoute(c net.Conn) route {
	raddr := c.RemoteAddr()
	nwk := raddr.Network()
	switch nwk {
	case "unix":
		return routeOSLocal
	case "tcp", "tcp4", "tcp6":
		a, err := net.ResolveIPAddr(nwk, raddr.String())
		if err != nil {
			return routeUnknown
		}
		if a.IP.IsLoopback() {
			return routeOSLocal
		} else if a.IP.IsLinkLocalUnicast() {
			return routeLinkLocal
		} else if a.IP.IsGlobalUnicast() {
			return routeGlobal
		}
		fallthrough
	default:
		return routeUnknown
	}
}

// connHandler handles network
// connections and multiplexes requests
// to connWrappers
type connHandler struct {
	svcname string
	h       Handler
	conn    net.Conn
	remote  net.Addr
	wg      sync.WaitGroup    // outstanding handlers
	writing chan *connWrapper // write queue
	route   route             // from whence?
}

func (c *connHandler) writeLoop() error {
	bwr := fwd.NewWriterSize(c.conn, 4096)

	// this works the same way
	// as (*client).writeLoop()
	//
	// the goroutine wakes up when
	// there are pending messages
	// to write. it writes the first
	// one into the buffered writer,
	// and continues to fill the
	// buffered writer until
	// no more messages are pending,
	// and then flushes whatever is
	// left. (*connHandler).writing
	// is closed when there are no
	// more pending handlers.
	var err error
	for {
		cw, ok := <-c.writing
		if !ok {
			// this is the "normal"
			// exit point for this
			// goroutine.
			return nil
		}
		if !c.do(bwr.Write(cw.res.out)) {
			goto flush
		}

		wrappers.push(cw)
	more:
		select {
		case another, ok := <-c.writing:
			if ok {
				f := c.do(bwr.Write(another.res.out))
				wrappers.push(another)
				if !f {
					goto flush
				}
				goto more
			} else {
				bwr.Flush()
				return nil
			}
		default:
			if !c.do(0, bwr.Flush()) {
				goto flush
			}
		}
	}
flush:
	for w := range c.writing {
		wrappers.push(w)
	}
	return err
}

func (c *connHandler) do(i int, err error) bool {
	if err != nil {
		c.conn.Close()
		return false
	}
	return true
}

// connLoop continuously polls the connection.
// requests are read synchronously; the responses
// are written in a spawned goroutine
func (c *connHandler) connLoop() {
	brd := fwd.NewReaderSize(c.conn, 4096)

	var (
		lead  [leadSize]byte
		seq   uint64
		sz    int
		frame fType
		err   error
	)

	for {
		// loop:
		//  - read seq, type, sz
		//  - call handler asynchronously

		if !c.do(brd.ReadFull(lead[:])) {
			return
		}
		seq, frame, sz = readFrame(lead)

		// handle commands
		if frame == fCMD {
			var body []byte // command body; may be nil
			var cmd command // command byte

			// the 1-byte body case
			// is pretty common for fCMD
			if sz == 1 {
				var bt byte
				bt, err = brd.ReadByte()
				cmd = command(bt)
			} else {
				body = make([]byte, sz)
				_, err = brd.ReadFull(body)
				cmd = command(body[0])
				body = body[1:]
			}
			if err != nil {
				c.conn.Close()
				return
			}
			c.wg.Add(1)
			go handleCmd(c, seq, cmd, body)
			continue
		}

		// the only valid frame
		// type left is fREQ
		if frame != fREQ {
			if !c.do(brd.Skip(sz)) {
				return
			}
			continue
		}

		w := wrappers.pop()

		if cap(w.in) >= sz {
			w.in = w.in[0:sz]
		} else {
			w.in = make([]byte, sz)
		}

		if !c.do(brd.ReadFull(w.in)) {
			return
		}

		// trigger handler
		w.seq = seq
		c.wg.Add(1)
		go c.handleReq(w)
	}
}

// connWrapper contains all the resources
// necessary to execute a Handler on a request
type connWrapper struct {
	next *connWrapper // only used by slab
	seq  uint64       // sequence number
	req  request      // (8w)
	res  response     // (4w)
	in   []byte       // incoming message
}

// handleconn sets up the Request and ResponseWriter
// interfaces and calls the handler.
func (c *connHandler) handleReq(cw *connWrapper) {
	// clear/reset everything
	cw.req.addr = c.remote
	cw.res.wrote = false

	var err error

	// split request into 'name' and body
	cw.req.mtd, cw.req.in, err = msgp.ReadUint32Bytes(cw.in)
	if err != nil {
		cw.res.Error(StatusBadRequest, "malformed request method")
	} else {
		c.h.ServeCall(&cw.req, &cw.res)
		// if the handler didn't write a body,
		// write 'nil'
		if !cw.res.wrote {
			cw.res.Send(nil)
		}
	}

	blen := len(cw.res.out) - leadSize // length minus frame length
	putFrame(cw.res.out, cw.seq, fRES, blen)
	c.writing <- cw
	c.wg.Done()
}

func handleCmd(c *connHandler, seq uint64, cmd command, body []byte) {
	resbyte := byte(cmd)
	var res []byte
	var err error
	if cmd == cmdInvalid || cmd >= _maxcommand {
		resbyte = byte(cmdInvalid)
	} else {
		res, err = cmdDirectory[cmd].handle(c, body)
		if err != nil {
			resbyte = byte(cmdInvalid)
		}
	}

	// for now, we'll use one of the
	// connection wrappers
	wr := wrappers.pop()

	sz := len(res) + 1
	need := sz + leadSize
	if cap(wr.res.out) < need {
		wr.res.out = make([]byte, need)
	} else {
		wr.res.out = wr.res.out[:need]
	}

	putFrame(wr.res.out[:], seq, fCMD, sz)
	wr.res.out[leadSize] = resbyte
	if res != nil {
		copy(wr.res.out[leadSize+1:], res)
	}
	c.writing <- wr
	c.wg.Done()
}
