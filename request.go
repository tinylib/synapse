package synapse

import (
	"github.com/philhofer/msgp/msgp"
	"net"
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

	// IsNil returns whether or not
	// the body of the request is 'nil'
	IsNil() bool
}

// Request implementation
type request struct {
	name string
	in   []byte
	addr net.Addr
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

func (r *request) IsNil() bool {
	return msgp.IsNil(r.in)
}
