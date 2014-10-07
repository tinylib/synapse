package synapse

// Handler is the interface that
// is satisfied by handlers to
// a particular method name
type Handler interface {

	// ServeCall handles a synapse request and
	// writes a response. It should be safe to
	// call ServeCall from multiple goroutines.
	// Both the request and response interfaces
	// become invalid after the call is returned;
	// no implementation of ServeCall should
	// maintain a reference to either object.
	ServeCall(req Request, res ResponseWriter)
}

type handlerFunc func(req Request, res ResponseWriter)

func (f handlerFunc) ServeCall(req Request, res ResponseWriter) {
	f(req, res)
}

// Status represents
// a response status code
type Status int

// These are the status
// codes that can be passed
// to a ResponseWriter as
// an error to send to the
// client.
const (
	// these are for
	// internal use;
	// 'ok' is not
	// an error
	invalidStatus Status = iota
	okStatus

	// NotFound means the
	// call requested could
	// not be located on the
	// server
	NotFound

	// BadRequest means the
	// request was not formatted
	// as expected
	BadRequest

	// NotAuthed means the client
	// was not authorized to make
	// the call
	NotAuthed

	// ServerError means the server
	// encountered an error when
	// making the call
	ServerError
)

// Status implements the error interface
func (s Status) Error() string {
	switch s {
	case okStatus:
		return ""
	case NotFound:
		return "not found"
	case BadRequest:
		return "bad request"
	case NotAuthed:
		return "not authorized"
	case ServerError:
		return "server error"
	default:
		return "invalid status"
	}
}
