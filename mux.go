package synapse

func NewMux() Mux {
	return &mux{
		hlrs: make(map[string]Handler),
	}
}

type Mux interface {
	// Register registers a handler
	// by with a give name
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
		res.Deny(NotFound)
	}
	h.ServeCall(req, res)
}
