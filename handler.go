package synapse

// Handler is the interface that
// is satisfied by handlers to
// a particular method name
type Handler interface {

	// ServeCall handles a synapse request and
	// writes a response. It should be safe to
	// call ServeCall from multiple goroutines.
	ServeCall(req Request, res ResponseWriter)
}

type handlerFunc func(req Request, res ResponseWriter)

func (f handlerFunc) ServeCall(req Request, res ResponseWriter) {
	f(req, res)
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

// Status implements the error interface
func (s Status) Error() string {
	switch s {
	case OK:
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
		return "unknown status"
	}
}
