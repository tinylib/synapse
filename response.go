package synapse

import (
	"github.com/philhofer/msgp/enc"
)

// ResponseWriter implementation
type response struct {
	status Status
	wrote  bool
	en     *enc.MsgWriter
	err    error
}

func (r *response) WriteHeader(s Status) {
	if r.wrote {
		return
	}
	r.status = s
	return
}

func (r *response) Send(e enc.MsgEncoder) {
	if r.wrote {
		return
	}
	r.wrote = true
	if r.status == Invalid {
		r.status = OK
	}
	_, r.err = r.en.WriteInt(int(r.status))
	if r.err != nil {
		return
	}
	if e != nil {
		_, r.err = e.EncodeTo(r.en)
	} else {
		_, r.err = r.en.WriteMapHeader(0)
	}
	return
}
