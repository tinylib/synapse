package synapse

import (
	"errors"
	"github.com/philhofer/msgp/enc"
)

var (
	// ErrTooLarge is returned when the message
	// size is larger than 65535 bytes.
	ErrTooLarge = errors.New("message body too large")
)

// A ResponseWriter it the interface
// with which servers write responses
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
	wrote bool
	en    *enc.MsgWriter
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
	nr, _ = r.en.WriteInt(int(OK))
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
