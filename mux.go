package synapse

// NewMux returns a mux that registers
// named handlers. Mux will pass
// requests to the handler that has exactly
// the same name as the requested method. Otherwise,
// it will return a Not Found error to the client.
func NewMux() Mux {
	return &mux{
		hlrs: make(map[string]Handler),
	}
}

// Mux is a server request multiplexer
type Mux interface {
	// Register registers a handler
	// by 'name'
	Register(name string, handler Handler)

	// Mux implements the Handler interface
	ServeCall(req Request, res ResponseWriter)
}

// this is a map for now; hopefully
// a radix tree in the future
type mux struct {
	hlrs map[string]Handler
}

func (m *mux) Register(name string, handler Handler) {
	m.hlrs[name] = handler
}

func (m *mux) ServeCall(req Request, res ResponseWriter) {
	h, ok := m.hlrs[req.Name()]
	if !ok {
		res.WriteHeader(NotFound)
	}
	h.ServeCall(req, res)
}
