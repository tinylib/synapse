package synapse

import (
	"github.com/inconshreveable/muxado"
	"github.com/philhofer/msgp/enc"
	"github.com/philhofer/netpewl"
	"net"
	"sync"
)

type streamGen struct {
	session muxado.Session
}

// DialNew implements the netpewl.Generator interface
func (s *streamGen) DialNew() (net.Conn, error) {
	return s.session.Open()
}

// returns
func cache(s muxado.Session) netpewl.Pool {
	return netpewl.New(&streamGen{session: s}, 512)
}

type Client struct {
	sess     muxado.Session
	sessions sync.Pool
	streams  netpewl.Pool
}

func Dial(addr string) (*Client, error) {
	sess, err := muxado.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	cl := &Client{}

	cl.sess = sess
	cl.streams = cache(sess)

	// the 'sessions' pool either returns
	// *clientconn or error
	cl.sessions.New = func() interface{} {
		conn, err := cl.streams.Pop()
		if err != nil {
			return err
		}
		return newclient(conn)
	}
	return cl, nil
}

func (c *Client) Call(method string, in enc.MsgEncoder, out enc.MsgDecoder) error {
	m := c.sessions.Get()
	// 'm' can be either *clientconn or error

	// pop session; call 'Call'
	if cl, ok := m.(*clientconn); ok {
		err := cl.Call(method, in, out)
		c.sessions.Put(cl)
		return err
	}

	return m.(error)
}

func (c *Client) Close() error {
	c.sessions = sync.Pool{}
	return c.sess.Close()
}
