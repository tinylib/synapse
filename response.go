package synapse

import (
	"github.com/tinylib/msgp/msgp"
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
	Send(msgp.Marshaler) error
}

// ResponseWriter implementation
type response struct {
	out   []byte  // body
	wrote bool    // written?
	_     [7]byte // pad
}

func (r *response) resetLead() {
	// we need to save the lead bytes
	if cap(r.out) < leadSize {
		r.out = make([]byte, leadSize, 256)
	} else {
		r.out = r.out[0:leadSize]
	}
}

func (r *response) Error(s Status) {
	if r.wrote {
		return
	}
	r.wrote = true
	r.resetLead()
	r.out = msgp.AppendInt(r.out, int(s))
}

func (r *response) Send(msg msgp.Marshaler) error {
	if r.wrote {
		return nil
	}
	r.wrote = true

	var err error
	r.resetLead()
	r.out = msgp.AppendInt(r.out, int(okStatus))
	if msg != nil {
		r.out, err = msg.MarshalMsg(r.out)
		if err != nil {
			return err
		}
		if len(r.out) > maxMessageSize {
			return ErrTooLarge
		}
		return nil
	}
	r.out = msgp.AppendNil(r.out)
	return nil
}
