package synapse

import (
	"github.com/philhofer/msgp/enc"
	"io"
)

// Handler is the interface that
// is satisfied by handlers to
// a particular method name
type Handler interface {
	ServeCall(req Request, res ResponseWriter)
}

// Request is a request for data
type Request interface {
	// Name returns the name of
	// the requested method
	Name() string

	// RemoteAddr returns the address
	// that the request originated from
	RemoteAddr() string

	// Decode reads the data of the request
	// into 'rf'
	Decode(enc.MsgDecoder) error
}

// ResponeWriter ...
type ResponseWriter interface {
	// Deny denies the request
	// for the reason supplied
	// in the argument
	WriteHeader(Status)

	// Send sends the argument
	// to the requester
	Send(enc.MsgEncoder)
}

type Status int

const (
	Invalid Status = iota
	OK
	NotFound
	NotAuthed
	ServerError
)
