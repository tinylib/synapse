package synapse

// NewRouter returns a new, empty router.
func NewRouter() *Router {
	return &Router{hlrs: make(map[string]Handler)}
}

// Router is a request router. It
// matches request names to handlers.
// Request names must precisely match
// the names of the handlers.
type Router struct {
	hlrs map[string]Handler
}

// Handle adds a named handler to the router.
func (r *Router) Handle(name string, handler Handler) {
	r.hlrs[name] = handler
}

// HandleFunc adds a new handler to the router.
func (r *Router) HandleFunc(name string, f func(Request, ResponseWriter)) {
	r.hlrs[name] = handlerFunc(f)
}

// shim for handlers as functions
type handlerFunc func(req Request, res ResponseWriter)

func (f handlerFunc) ServeCall(req Request, res ResponseWriter) { f(req, res) }

// ServeCall implements the Handler interface.
func (r *Router) ServeCall(req Request, res ResponseWriter) {
	h, ok := r.hlrs[req.Name()]
	if !ok {
		res.Error(StatusNotFound, "no handler by that name")
		return
	}
	h.ServeCall(req, res)
}
