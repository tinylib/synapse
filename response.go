package synapse

import "github.com/philhofer/msgp/msgp"

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
	en    *msgp.Writer
	wrote bool
}

func (r *response) Error(s Status) {
	if r.wrote {
		return
	}
	r.wrote = true
	r.en.WriteInt(int(s))
}

func (r *response) Send(e msgp.Marshaler) error {
	if r.wrote {
		return nil
	}
	r.wrote = true

	var err error
	err = r.en.WriteInt(int(okStatus))
	if err != nil {
		return err
	}
	if e != nil {
		err = r.en.Encode(e)
		return err
	}
	r.en.WriteNil()
	return nil
}
