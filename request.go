package synapse

import (
	"github.com/philhofer/msgp/enc"
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
	Decode(enc.MsgDecoder) error
}

// Request implementation
type request struct {
	name string
	addr net.Addr
	dc   *enc.MsgReader
}

func (r *request) refresh() error {
	var err error
	r.name, _, err = r.dc.ReadString()
	return err
}

func (r *request) Name() string         { return r.name }
func (r *request) RemoteAddr() net.Addr { return r.addr }

func (r *request) Decode(m enc.MsgDecoder) error {
	var err error
	if m != nil {
		_, err = m.DecodeFrom(r.dc)
		return err
	}
	_, err = r.dc.Skip()
	return err
}
