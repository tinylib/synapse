package synapse

import (
	"net"

	"github.com/philhofer/msgp/msgp"
)

// Request is the interface that
// Handlers use to interact with
// requests.
type Request interface {
	// Name returns the name of
	// the requested method
	Name() string

	// RemoteAddr returns the address
	// that the request originated from
	RemoteAddr() net.Addr

	// Decode reads the data of the request
	// into the argument.
	Decode(msgp.Unmarshaler) error
}

// Request implementation
type request struct {
	name string
	addr net.Addr
	in   []byte
}

func (r *request) Name() string         { return r.name }
func (r *request) RemoteAddr() net.Addr { return r.addr }

func (r *request) Decode(m msgp.Unmarshaler) error {
	var err error
	if m != nil {
		r.in, err = m.UnmarshalMsg(r.in)
		return err
	}
	return nil
}
