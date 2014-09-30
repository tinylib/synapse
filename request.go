package synapse

import (
	"github.com/philhofer/msgp/enc"
	"net"
)

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
	_, err := m.DecodeFrom(r.dc)
	return err
}
