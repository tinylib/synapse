package synapse

import (
	"github.com/philhofer/msgp/enc"
)

// A ResponseWriter is the interface through
// which Handlers write responses. Handlers can
// either return a body (through Send) or an
// error (through Error); whichever call is
// made first wins.
type ResponseWriter interface {
	// Error writes an error status
	// to the client. Any following
	// calls to Error or Send are no-ops.
	Error(Status)

	// Send sends the argument
	// to the requester. Additional calls
	// to send no-op. Send will error if the
	// encoder errors or the message size
	// is too large.
	Send(enc.MsgEncoder) error
}

// ResponseWriter implementation
type response struct {
	en    *enc.MsgWriter
	wrote bool
}

func (r *response) Error(s Status) {
	if r.wrote {
		return
	}
	r.wrote = true
	r.en.WriteInt(int(s))
}

func (r *response) Send(e enc.MsgEncoder) error {
	if r.wrote {
		return nil
	}
	r.wrote = true

	var nr int
	var nn int
	var err error
	nr, _ = r.en.WriteInt(int(okStatus))
	if e != nil {
		nn, err = e.EncodeTo(r.en)
		if err != nil {
			return err
		}
		nr += nn
		if nr+leadSize > maxMessageSize {
			return ErrTooLarge
		}
		return nil
	}
	r.en.WriteMapHeader(0)
	return nil
}
