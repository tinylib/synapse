package synapse

import (
	"net"
	"sync/atomic"
	"time"
)

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
type command byte

// an action is the consequence
// of a command - commands are
// mapped to actions
type action interface {
	// Client is the action carried out on the client side
	// when it receives a command response from a server
	Client(c *client, from net.Conn, msg []byte)

	// Sever is the action carried out on the server side. It
	// should return the reponse message (if any), and any
	// error encountered. Errors will result in cmdInvalid
	// sent to the client.
	Server(from net.Conn, msg []byte) (res []byte, err error)
}

// cmdDirectory is a map of all the commands
// to their respective actions
var cmdDirectory = map[command]action{
	cmdPing: ping{},
	cmdTime: logTime{},
}

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

	// timer is a
	// command to
	// gather connection
	// timing information
	cmdTime
)

// ping is a no-op on both sides
type ping struct{}

func (p ping) Client(_ *client, _ net.Conn, _ []byte) {}

func (p ping) Server(from net.Conn, _ []byte) ([]byte, error) { return nil, nil }

type logTime struct{}

func (t logTime) Client(cl *client, c net.Conn, res []byte) {
	var tm time.Time
	err := tm.UnmarshalBinary(res)
	if err != nil {
		return
	}
	d := time.Since(tm)
	atomic.StoreInt64(&cl.rtt, d.Nanoseconds())
}

func (t logTime) Server(from net.Conn, _ []byte) ([]byte, error) {
	return time.Now().MarshalBinary()
}
