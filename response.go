package synapse

import (
	"github.com/tinylib/msgp/msgp"
)

const outPrealloc = 256

// A ResponseWriter is the interface through
// which Handlers write responses. Handlers can
// either return a body (through Send) or an
// error (through Error); whichever call is
// made first should 'win'. Subsequent calls
// to either method should no-op.
type ResponseWriter interface {
	// Error sets an error status
	// to be returned to the caller,
	// along with an explanation
	// of the error.
	Error(Status, string)

	// Send sets the body to be
	// returned to the caller.
	Send(msgp.Marshaler) error
}

// ResponseWriter implementation
type response struct {
	out   []byte              // body
	wrote bool                // written?
	_     [sizeofPtr - 1]byte // pad
}

func (r *response) resetLead() {
	// we need to save the lead bytes
	if cap(r.out) < leadSize {
		r.out = make([]byte, leadSize, outPrealloc)
		return
	}
	r.out = r.out[0:leadSize]
	return
}

// base Error implementation
func (r *response) Error(s Status, expl string) {
	if r.wrote {
		return
	}
	r.wrote = true
	r.resetLead()
	r.out = msgp.AppendInt(r.out, int(s))
	r.out = msgp.AppendString(r.out, expl)
}

// base Send implementation
func (r *response) Send(msg msgp.Marshaler) error {
	if r.wrote {
		return nil
	}
	r.wrote = true

	var err error
	r.resetLead()
	r.out = msgp.AppendInt(r.out, int(StatusOK))
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
