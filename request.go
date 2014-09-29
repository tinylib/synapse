package synapse

import (
	"github.com/philhofer/msgp/enc"
)

// request implementation
type request struct {
	name string
	addr string
	dc   *enc.MsgReader
}

func (r *request) refresh() error {
	var err error
	r.name, _, err = r.dc.ReadString()
	return err
}

func (r *request) Name() string       { return r.name }
func (r *request) RemoteAddr() string { return r.addr }

func (r *request) Decode(m enc.MsgDecoder) error {
	_, err := r.dc.ReadIdent(m)
	return err
}
