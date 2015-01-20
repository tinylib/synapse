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

// cmdDirectory is a map of all the commands
// to their respective actions
var cmdDirectory = [_maxcommand]action{
	cmdPing: ping{},
}

// an action is the consequence
// of a command - commands are
// mapped to actions
type action interface {
	// Client is the action carried out on the client side
	// when it receives a command response from a server
	Client(c *Client, msg []byte)

	// Sever is the action carried out on the server side. It
	// should return the reponse message (if any), and any
	// error encountered. Errors will result in cmdInvalid
	// sent to the client.
	Server(ch *connHandler, msg []byte) (res []byte, err error)
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

	// a command >= _maxcommand
	// is invalid
	_maxcommand
)

// ping is a no-op on both sides
type ping struct{}

func (p ping) Client(cl *Client, res []byte) {}

func (p ping) Server(ch *connHandler, body []byte) ([]byte, error) { return nil, nil }
