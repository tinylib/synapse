package synapse

import (
	"fmt"
)

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
	StatusInvalid     Status = iota // zero-value for Status
	StatusOK                        // OK
	StatusNotFound                  // no handler for the request method
	StatusCondition                 // precondition failure
	StatusBadRequest                // mal-formed request
	StatusNotAuthed                 // not authorized
	StatusServerError               // server-side error
	StatusOther                     // other error
)

// ResponseError is the type of error
// returned by the client when the
// server sends a response with
// ResponseWriter.Error()
type ResponseError struct {
	Code Status
	Expl string
}

// Error implements error
func (e *ResponseError) Error() string {
	return fmt.Sprintf("synapse: response error (%s): %s", e.Code, e.Expl)
}

// String returns the string representation of the status
func (s Status) String() string {
	switch s {
	case StatusOK:
		return "OK"
	case StatusNotFound:
		return "not found"
	case StatusCondition:
		return "precondition failed"
	case StatusBadRequest:
		return "bad request"
	case StatusNotAuthed:
		return "not authorized"
	case StatusServerError:
		return "server error"
	case StatusOther:
		return "other"
	default:
		return fmt.Sprintf("Status(%d)", s)
	}
}
