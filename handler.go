package synapse

import (
	"github.com/philhofer/msgp/enc"
	"net"
)

// Client is the interface fulfilled
// by synapse clients.
type Client interface {
	// Call asks the server to perform 'method' on 'in' and
	// return the response to 'out'.
	Call(method string, in enc.MsgEncoder, out enc.MsgDecoder) error

	// Close closes the client.
	Close() error

	// ForceClose closes the client immediately.
	ForceClose() error
}

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
	RemoteAddr() net.Addr

	// Decode reads the data of the request
	// into 'rf'
	Decode(enc.MsgDecoder) error
}

// A ResponseWriter it the interface
// with which servers write responses
type ResponseWriter interface {
	// Deny denies the request
	// for the reason supplied
	// in the argument
	WriteHeader(Status)

	// Send sends the argument
	// to the requester
	Send(enc.MsgEncoder)
}

// Status represents
// a response status code
type Status int

// status codes
const (
	Invalid Status = iota
	OK
	NotFound
	BadRequest
	NotAuthed
	ServerError
)
