package synapse

import (
	"bytes"
	"github.com/tinylib/msgp/msgp"
	"log"
	"net"
	"testing"
)

type logger interface {
	Print(...interface{})
	Printf(string, ...interface{})
}

type debugh struct {
	inner  Handler
	logger logger
}

// shim for *testing.T
// to implement logger
type tlog testing.T

func (t *tlog) Print(v ...interface{})            { (*testing.T)(t).Log(v...) }
func (t *tlog) Printf(s string, v ...interface{}) { (*testing.T)(t).Logf(s, v...) }

// Debug wraps a handler and logs all of the incoming
// and outgoing data using the provided logger.
func Debug(h Handler, l *log.Logger) Handler {
	return &debugh{inner: h, logger: l}
}

// DebugTest works identically to Debug, except
// that it logs data to a *testing.T.
//
// (DebugTest only calls t.Log/f; it never calls t.Error/f
// or t.Fatal/f.)
func DebugTest(h Handler, t *testing.T) Handler {
	return &debugh{inner: h, logger: (*tlog)(t)}
}

func (d *debugh) ServeCall(req Request, res ResponseWriter) {
	// if we're handed the base types
	// used by the package, we can do this
	// with a lot less overhead
	if rq, ok := req.(*request); ok {
		if rs, ok := res.(*response); ok {
			d.serveBase(rq, rs)
			return
		}
	}

	var buf bytes.Buffer
	nm := req.Name()
	remote := req.RemoteAddr()

	var raw msgp.Raw
	err := req.Decode(&raw)
	if err != nil {
		d.logger.Printf("request from %s for %q is malformed: %s\n", remote, nm, err)
		res.Error(StatusBadRequest, err.Error())
		return
	}
	msgp.UnmarshalAsJSON(&buf, []byte(raw))
	d.logger.Printf("request from %s:\n\tNAME: %q\n\tBODY: %s\n", remote, nm, buf.String())
	buf.Reset()
	r := &mockReq{
		name:   nm,
		remote: remote,
		raw:    raw,
	}
	w := &mockRes{}
	d.inner.ServeCall(r, w)
	if !w.wrote {
		d.logger.Print("WARNING: handler for", nm, "did not write a valid response")
		res.Error(StatusServerError, "empty response")
		return
	}
	msgp.UnmarshalAsJSON(&buf, []byte(w.out))
	d.logger.Printf("response to %s for %q:\n\tSTATUS: %s\n\tBODY: %s\n", remote, nm, w.status, buf.String())
	res.Send(msgp.Raw(w.out))
}

type mockReq struct {
	name   string
	remote net.Addr
	raw    msgp.Raw
}

func (m *mockReq) IsNil() bool          { return msgp.IsNil([]byte(m.raw)) }
func (m *mockReq) Name() string         { return m.name }
func (m *mockReq) RemoteAddr() net.Addr { return m.remote }
func (m *mockReq) Decode(u msgp.Unmarshaler) error {
	_, err := u.UnmarshalMsg([]byte(m.raw))
	return err
}

type mockRes struct {
	wrote  bool
	out    []byte
	status Status
}

func (m *mockRes) Send(g msgp.Marshaler) error {
	if m.wrote {
		return nil
	}
	m.wrote = true
	var err error
	m.out, err = g.MarshalMsg(nil)
	if err != nil {
		return err
	}
	m.status = StatusOK
	return nil
}

func (m *mockRes) Error(s Status, r string) {
	if m.wrote {
		return
	}
	m.wrote = true
	m.status = s
	m.out = msgp.AppendString(m.out, r)
}

func (d *debugh) serveBase(req *request, res *response) {
	var buf bytes.Buffer
	_, err := msgp.UnmarshalAsJSON(&buf, req.in)
	remote := req.addr.String()
	if err != nil {
		d.logger.Printf("request from %s for %q was malformed: %s", remote, req.name, err)
		res.Error(StatusBadRequest, err.Error())
		return
	}
	d.logger.Printf("request from %s:\n\tNAME: %q\n\tBODY: %s\n", remote, req.name, buf.String())
	buf.Reset()
	d.inner.ServeCall(req, res)
	if !res.wrote || len(res.out) < leadSize {
		d.logger.Print("WARNING: handler for", req.name, "did not write a valid response")
		res.Error(StatusServerError, "empty response")
		return
	}
	out := res.out[leadSize:]
	var stat int
	var body []byte
	stat, body, err = msgp.ReadIntBytes(out)
	if err != nil {
		d.logger.Printf("body of response to %s is malformed: %s", req.name, err)
		return
	}
	status := Status(stat)
	_, err = msgp.UnmarshalAsJSON(&buf, body)
	if err != nil {
		d.logger.Printf("response for %s is malformed: %s", req.name, err)
		return
	}
	d.logger.Printf("response to %s for %q:\n\tSTATUS: %s\n\tBODY: %s\n", remote, req.name, status, buf.String())
}
