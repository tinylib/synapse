package synapse

// Handler is the interface used
// to register server response callbacks.
type Handler interface {

	// ServeCall handles a synapse request and
	// writes a response. It should be safe to
	// call ServeCall from multiple goroutines.
	// Both the request and response interfaces
	// become invalid after the call is returned;
	// no implementation of ServeCall should
	// maintain a reference to either object
	// after the function returns.
	ServeCall(req Request, res ResponseWriter)
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
	// InvalidStatus is the
	// zero value of Status.
	InvalidStatus Status = iota

	// un-exported, because 'ok'
	// is not really an error
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
		return "<ok>"
	case NotFound:
		return "not found"
	case BadRequest:
		return "bad request"
	case NotAuthed:
		return "not authorized"
	case ServerError:
		return "server error"
	default:
		return "<invalid status>"
	}
}
