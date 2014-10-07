package synapse

// NewRouter returns a mux that registers
// named handlers. Mux will pass
// requests to the handler that has exactly
// the same name as the requested method. Otherwise,
// it will return `synapse.NotFound` to the client.
func NewRouter() Router {
	return &mux{hlrs: make(map[string]Handler)}
}

// Router is a server request
// multiplexer
type Router interface {
	// Handle registers a handler. Handle should
	// not be called after a server has started
	// serving the router.
	Handle(name string, handler Handler)

	// HandleFunc registers a handler. HandleFunc
	// should not be called after a server has
	// started serving the router.
	HandleFunc(name string, f func(Request, ResponseWriter))

	// Routers implement the Handler interface
	ServeCall(req Request, res ResponseWriter)
}

// this is a map for now; hopefully
// a radix tree in the future
type mux struct {
	hlrs map[string]Handler
}

func (m *mux) Handle(name string, handler Handler) {
	m.hlrs[name] = handler
}

func (m *mux) HandleFunc(name string, f func(Request, ResponseWriter)) {
	m.hlrs[name] = handlerFunc(f)
}

func (m *mux) ServeCall(req Request, res ResponseWriter) {
	h, ok := m.hlrs[req.Name()]
	if !ok {
		res.Error(NotFound)
		return
	}
	h.ServeCall(req, res)
}
