package synapse

import (
	"github.com/philhofer/msgp/enc"
)

// A ResponseWriter it the interface
// with which servers write responses
type ResponseWriter interface {
	// WriteHeader writes the status
	// of the response. It is not necessary
	// to call WriteHeader if the status
	// is OK. Calls to WriteHeader after
	// calls to Send() no-op.
	WriteHeader(Status)

	// Send sends the argument
	// to the requester. Additional calls
	// to send no-op.
	Send(enc.MsgEncoder)
}

// ResponseWriter implementation
type response struct {
	status Status
	wrote  bool
	en     *enc.MsgWriter
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
	r.en.WriteInt(int(r.status))
	if e != nil {
		e.EncodeTo(r.en)
		return
	}
	r.en.WriteMapHeader(0)
	return
}
