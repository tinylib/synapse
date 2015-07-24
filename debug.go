package synapse

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/tinylib/msgp/msgp"
)

var methodTab = map[Method]string{}

func RegisterName(m Method, name string) {
	methodTab[m] = name
}

func (m Method) String() string {
	if str, ok := methodTab[m]; ok {
		return str
	}
	return fmt.Sprintf("Method(%d)", m)
}

type logger interface {
	Print(...interface{})
	Printf(string, ...interface{})
}

type debugh struct {
	inner  Handler
	logger logger
}

// Debug wraps a handler and logs all of the incoming
// and outgoing data using the provided logger.
func Debug(h Handler, l *log.Logger) Handler {
	return &debugh{inner: h, logger: l}
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
	nm := req.Method()
	remote := req.RemoteAddr()

	var raw msgp.Raw
	err := req.Decode(&raw)
	if err != nil {
		d.logger.Printf("request from %s for method %s is malformed: %s\n", remote, nm, err)
		res.Error(StatusBadRequest, err.Error())
		return
	}
	msgp.UnmarshalAsJSON(&buf, []byte(raw))
	d.logger.Printf("request from %s:\n\tMETHOD: %s\n\tBODY: %s\n", remote, nm, buf.Bytes())
	buf.Reset()
	r := &mockReq{
		mtd:    nm,
		remote: remote,
		raw:    raw,
	}
	w := &mockRes{}
	start := time.Now()
	d.inner.ServeCall(r, w)
	ctime := time.Since(start)
	if !w.wrote {
		d.logger.Print("WARNING: handler for", nm, "did not write a valid response")
		res.Error(StatusServerError, "empty response")
		return
	}
	msgp.UnmarshalAsJSON(&buf, []byte(w.out))
	d.logger.Printf("response to %s for method %s:\n\tSTATUS: %s\n\tBODY: %s\n\tDURATION: %s", remote, nm, w.status, buf.Bytes(), ctime)
	res.Send(msgp.Raw(w.out))
}

type mockReq struct {
	remote net.Addr
	raw    msgp.Raw
	mtd    Method
}

func (m *mockReq) IsNil() bool          { return msgp.IsNil([]byte(m.raw)) }
func (m *mockReq) Method() Method       { return m.mtd }
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
		d.logger.Printf("request from %s for %s was malformed: %s", remote, Method(req.mtd), err)
		res.Error(StatusBadRequest, err.Error())
		return
	}
	d.logger.Printf("request from %s:\n\tMETHOD: %s\n\tREQUEST BODY: %s\n", remote, Method(req.mtd), buf.Bytes())
	buf.Reset()
	start := time.Now()
	d.inner.ServeCall(req, res)
	ctime := time.Since(start)
	if !res.wrote || len(res.out) < leadSize {
		d.logger.Print("WARNING: handler for", Method(req.mtd), "did not write a valid response")
		res.Error(StatusServerError, "empty response")
		return
	}
	out := res.out[leadSize:]
	var stat int
	var body []byte
	stat, body, err = msgp.ReadIntBytes(out)
	if err != nil {
		d.logger.Printf("body of response to %s is malformed: %s", Method(req.mtd), err)
		return
	}
	status := Status(stat)
	_, err = msgp.UnmarshalAsJSON(&buf, body)
	if err != nil {
		d.logger.Printf("response for %s is malformed: %s", Method(req.mtd), err)
		return
	}
	d.logger.Printf("response to %s for %s:\n\tSTATUS: %s\n\tRESPONSE BODY: %s\n\tDURATION: %s\n", remote, Method(req.mtd), status, buf.Bytes(), ctime)
}
