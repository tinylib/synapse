package synapse

import (
	"log"
	"net"
	"syscall"
	"time"
)

// This file contains the error handlers
// for dial errors and network i/o errors.
// The goal here is to determine which
// errors warrant permanently dropping
// a connection vs. continually re-dialing.

const (
	// starting redial wait, in seconds
	startWait = 3 * time.Second

	// maximum redial wait, in seconds;
	// after backoff reaches this point,
	// we drop the connection permanently
	maxWait = 600 * time.Second
)

// handleDialError determines whether or not
// it should continue to dial the remote address or
// drop it permanently. doing nothing ignores the error.
func (c *clusterClient) handleDialError(addr string, err error) {
	// simplest case: temporary net error
	if neterr, ok := err.(net.Error); ok {
		if neterr.Temporary() {
			go c.redialPauseLoop(addr)
			return
		}
	}

	if operr, ok := err.(*net.OpError); ok {
		if _, ok := operr.Err.(syscall.Errno); ok {
			err = operr.Err
			goto errno
		}
	}

	// TODO: validate that these are
	// actually the responses we want
	// to have under these conditions.
errno:
	if errno, ok := err.(syscall.Errno); ok {
		if !errno.Temporary() {
			switch errno {
			// we don't need to handle
			// the "drop permanently" case,
			// because that is the default behavior.

			// redial w/ backoff
			case syscall.EPIPE, syscall.ESHUTDOWN, syscall.ESTALE, syscall.ETIMEDOUT, syscall.ECONNREFUSED:
				go c.redialPauseLoop(addr)
				return

			}
		}
	}

	log.Printf("synapse cluster: permanently dropping remote @ %s %s: %s", c.nwk, addr, err)
}

// redialPauseLoop is called to wait/loop on dialing
// a problematic address
func (c *clusterClient) redialPauseLoop(addr string) {
	// TODO: smarter backoff
	//
	// right now we start at 3s and double until 600s, then break

	wait := startWait
	for {
		if wait > maxWait {
			log.Printf("synapse cluster: permanently dropping remote @ %s %s", c.nwk, addr)
			return
		}
		log.Printf("synapse cluster: redialing %s in %s", addr, wait)
		time.Sleep(wait)
		err := c.dial(c.nwk, addr, false)
		if err == nil {
			break
		}
		log.Printf("synapse cluster: error dialing %s: %s", addr, err)
		wait *= 2
	}
}

// handleErr is responsible for figuring out
// if an error warrants re-dialing a connection
func (c *clusterClient) handleErr(v *client, err error) {

	// ignore timeouts and user errors
	// from this package; they don't
	// indicate connection failure
	switch err {
	case nil, ErrTimeout, ErrTooLarge:
		return
	case ErrClosed:
		go c.redial(v)
		return
	}

	// ignore protocol-level errors
	if _, ok := err.(Status); ok {
		return
	}

	// most syscall-level errors are
	// returned as OpErrors by the net pkg
	if operr, ok := err.(*net.OpError); ok {
		if _, ok = operr.Err.(syscall.Errno); ok {
			err = operr.Err
			goto errno
		}
		if !operr.Temporary() {
			go c.redial(v)
			return
		}
		return
	}

	// this is the logic for handling
	// syscall-level errors; it is not
	// complete or exhaustive
errno:
	if errno, ok := err.(syscall.Errno); ok {

		// errors that are "temporary"
		// - EINTR (interrupted)
		// - EMFILE (out of file descriptors)
		// - ECONNRESET (connection reset)
		// - ECONNABORTED (connection aborted...?)
		// - EAGAIN (virtually never seen by clients of "net")
		// - EWOULDBLOCK (see above; POSIX says it may be identical to EAGAIN)
		// - ETIMEOUT (timeout)
		if !errno.Temporary() {
			switch errno {

			// errors for which we permanently drop the remote
			// TODO: validate
			case syscall.EACCES, syscall.EADDRINUSE, syscall.EADDRNOTAVAIL,
				syscall.EAFNOSUPPORT, syscall.EBADMSG, syscall.EDQUOT, syscall.EFAULT,
				syscall.EOPNOTSUPP, syscall.ESOCKTNOSUPPORT:
				if c.remove(v) {
					log.Printf("synapse cluster: permanently dropping remote @ %s: %s", v.conn.RemoteAddr(), err)
				}
				v.Close()
				return

			// redial loop
			case syscall.EHOSTDOWN, syscall.EHOSTUNREACH, syscall.EPIPE, syscall.ESHUTDOWN:

				// ensure idempotency
				if c.remove(v) {
					v.Close()
					go c.redialPauseLoop(v.conn.RemoteAddr().String())
				}

			}
		}
	}
}
