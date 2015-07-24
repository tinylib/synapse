package synapse

// RouteTable is the simplest and fastest
// form of routing. Request methods map
// directly to an index in the routing
// table.
//
// Route tables should be used by defining
// your methods in a const-iota block, and
// then using those as indices in the routing
// table.
type RouteTable []Handler

func (r *RouteTable) ServeCall(req Request, res ResponseWriter) {
	m := req.Method()
	if int(m) > len(*r) {
		res.Error(StatusNotFound, "no such method")
		return
	}
	h := (*r)[m]
	if h == nil {
		res.Error(StatusNotFound, "no such method")
		return
	}
	h.ServeCall(req, res)
	return
}
