package synapse

import (
	"bytes"
	"encoding/binary"
	"io"
	"log"
	"net"
	"sync/atomic"
)

// command is a message
// sent between the client
// and server (either direction)
// that allows either end
// to communicate information
// to the other
type command uint32

// an action is the consequence
// of a command - commands are
// mapped to actions
type action interface {
	// Client is the action carried out on the client side
	Client(c *client, from net.Conn, msg []byte)

	// Sever is the action carried out on the server side
	Server(from net.Conn, msg []byte)
}

// cmdDirectory is a map of all the commands
// to their respective actions
var cmdDirectory = map[command]action{
	cmdLog: logAction{},
	cmdFin: finAction{},
}

// Command protocol:
//
// |  seq   |   sz   |  cmd   |  msg  |
// | uint64 | uint32 | uint32 | (bts) |
//
// 'seq' is ALWAYS 0 (this is the special reserved sequence number for commands)
// 'sz' is the big-endian 32-bit unsigned size of the bytes remaining to be read
// 'cmd' is the 32-bit command signature
// 'msg' are optional bytes that have command-specific meaning
//
// Commands requests shouldn't block waiting for responses.

// list of commands
const (
	cmdInvalid command = iota

	// Log asks the remote
	// end to log a message.
	// There is no response
	// necessary.
	cmdLog

	// Fin tells the remote
	// end that the other
	// end of the pipe will be
	// closed. There is no
	// 'msg' field.
	cmdFin
)

// writeCmd writes the command to a connection
func writeCmd(conn net.Conn, cmd command, msg []byte) error {
	var buf bytes.Buffer
	var lead [16]byte
	binary.BigEndian.PutUint32(lead[8:12], uint32(len(msg)+4))
	binary.BigEndian.PutUint32(lead[12:16], uint32(cmd))

	buf.Write(lead[:])
	buf.Write(msg)

	_, err := conn.Write(buf.Bytes())
	return err
}

// readCmd reads a command from a connection
// assuming you've already read the seq(0) and size.
// 'conn' is passed to the action, but the message
// is read from 'r' (which may also be the conn, or
// a buffered reader)
func readCmdClient(c *client, conn net.Conn, r io.Reader, sz uint32) {
	bts := make([]byte, int(sz))
	if sz < 4 {
		return
	}
	_, err := io.ReadFull(r, bts)
	if err != nil {
		return
	}
	cmd := command(binary.BigEndian.Uint32(bts[0:4]))
	if cmd == cmdInvalid {
		return
	}
	act := cmdDirectory[cmd]
	if act == nil {
		return
	}
	act.Client(c, conn, bts[4:])
}

func readCmdServer(conn net.Conn, r io.Reader, sz uint32) {
	bts := make([]byte, int(sz))
	_, err := io.ReadFull(r, bts)
	if sz < 4 {
		return
	}
	if err != nil {
		return
	}
	cmd := command(binary.BigEndian.Uint32(bts[0:4]))
	if cmd == cmdInvalid {
		return
	}
	act := cmdDirectory[cmd]
	if act == nil {
		return
	}
	act.Server(conn, bts[4:])
}

// logAction is the response to cmdLog
type logAction struct{}

func (l logAction) do(c net.Conn, m []byte) {
	log.Printf("LOG from %s: %q", c.RemoteAddr(), m)
}
func (l logAction) Client(_ *client, c net.Conn, m []byte) { l.do(c, m) }
func (l logAction) Server(c net.Conn, m []byte)            { l.do(c, m) }

type finAction struct{}

func (f finAction) Client(c *client, conn net.Conn, _ []byte) {
	// set closed flag to 0; wait for server to close the connection
	if !atomic.CompareAndSwapUint32(&c.cflag, 1, 0) {
		return
	}
}

func (f finAction) Server(conn net.Conn, _ []byte) {
	log.Printf("FIN command received from %s", conn.RemoteAddr().String())
	// wait for remote end to hang up
}
