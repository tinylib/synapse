package synapse

import (
	"github.com/tinylib/msgp/msgp"
	"net"
)

// Request is the interface that
// Handlers use to interact with
// requests.
type Request interface {
	// Name returns the name of
	// the requested method.
	Name() string

	// RemoteAddr returns the remote
	// address that made the request.
	RemoteAddr() net.Addr

	// Decode reads the data of the request
	// into the argument.
	Decode(msgp.Unmarshaler) error

	// IsNil returns whether or not
	// the body of the request is 'nil'.
	IsNil() bool
}

// Request implementation passed
// to the root handler of the server.
type request struct {
	addr net.Addr // remote address
	name string   // method name
	in   []byte   // body
}

func (r *request) Name() string         { return r.name }
func (r *request) RemoteAddr() net.Addr { return r.addr }

func (r *request) Decode(m msgp.Unmarshaler) error {
	if m != nil {
		_, err := m.UnmarshalMsg(r.in)
		return err
	}
	return nil
}

func (r *request) IsNil() bool {
	return msgp.IsNil(r.in)
}
