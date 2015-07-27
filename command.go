package synapse

// Commands have request-response
// semantics similar to the user-level
// requests and responses. However, commands
// are only sent only by internal processes.
// Clients can use commands to poll for information
// from the server, etc.
//
// The ordering of events is:
//  - client -> writeCmd([command]) -> server -> action.Server() -> [command] -> client -> action.Client()
//
// See the ping handler and client.ping() for an example.

// frame type
type fType byte

//
const (
	// invalid frame
	fINVAL fType = iota

	// request frame
	fREQ

	// response frame
	fRES

	// command frame
	fCMD
)

// command is a message
// sent between the client
// and server (either direction)
// that allows either end
// to communicate information
// to the other
type command uint8

// Global directory of command handlers.
//
// If you need to implement a new command,
// add it here.
var cmdDirectory = [_maxcommand]cmdact{
	cmdInvalid:   {badhandle, recvbad},
	cmdPing:      {pinghandle, recvping},
	cmdListLinks: {sendlinks, recvlinks},
}

type cmdact struct {
	// handle is called by the server when
	// receiving a particular command
	handle func(ch *connHandler, msg []byte) ([]byte, error)

	// done is called as the client-side finalizer
	done func(c *Client, msg []byte)
}

// no-op handlers
func badhandle(ch *connHandler, msg []byte) ([]byte, error) {
	return []byte{byte(cmdInvalid)}, nil
}

func recvbad(c *Client, msg []byte) {}

// list of commands
const (
	// cmdInvalid is used
	// to indicate a command
	// doesn't exist or is
	// formatted incorrectly
	cmdInvalid command = iota

	// ping is a
	// simple ping
	// command
	cmdPing

	// sync service addresses
	// between client and server
	cmdListLinks

	// a command >= _maxcommand
	// is invalid
	_maxcommand
)

// client-side ping finalizer
func recvping(cl *Client, res []byte) {
	var s Service
	_, err := s.UnmarshalMsg(res)
	if err != nil {
		return
	}
	r := cl.conn.RemoteAddr()
	s.net = r.Network()
	s.addr = r.String()
	cache(&s)
	cl.svc = s.name
}

// server-side ping handler
func pinghandle(ch *connHandler, body []byte) ([]byte, error) {
	s := Service{
		name: string(ch.svcname),
		host: hostid,
	}
	return s.MarshalMsg(nil)
}

func recvlinks(cl *Client, res []byte) {
	var sl serviceList
	_, err := sl.UnmarshalMsg(res)
	if err != nil {
		return
	}
	svcCache.Lock()
	for _, sv := range sl {
		if sv.host == hostid || !isRoutable(sv) {
			continue
		}
		svcCache.tab[sv.name] = addSvc(svcCache.tab[sv.name], sv)
	}
	svcCache.Unlock()
}

func sendlinks(ch *connHandler, body []byte) ([]byte, error) {
	var sl serviceList
	_, err := sl.UnmarshalMsg(body)
	if err != nil {
		return nil, err
	}
	if ch.route != routeOSLocal {
		// for each non-os-local
		// service, increment the
		// hop counter
		for _, sv := range sl {
			sv.dist++
		}
	}
	svcCache.Lock()
	body, _ = svcCache.tab.MarshalMsg(body[:0])
	for _, sv := range sl {
		if sv.host == hostid {
			continue
		}
		svcCache.tab[sv.name] = addSvc(svcCache.tab[sv.name], sv)
	}
	svcCache.Unlock()
	return body, nil
}
