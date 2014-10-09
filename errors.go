package synapse

import (
	"log"
	"net"
	"syscall"
	"time"
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

errno:
	if errno, ok := err.(syscall.Errno); ok {
		if !errno.Temporary() {
			switch errno {
			case syscall.EALREADY:
				return

			case syscall.EACCES, syscall.EADDRINUSE, syscall.EADDRNOTAVAIL,
				syscall.EAFNOSUPPORT, syscall.EBADMSG, syscall.EDQUOT, syscall.EFAULT,
				syscall.EOPNOTSUPP, syscall.ESOCKTNOSUPPORT:
				log.Printf("synapse cluster: permanently dropping remote @ %s %s: %s", c.nwk, addr, err)
				return

			case syscall.EPIPE, syscall.ESHUTDOWN, syscall.ESTALE, syscall.ETIMEDOUT:
				go c.redialPauseLoop(addr)

			}
		}
	}

	log.Printf("synapse cluster: permanently dropping remote @ %s %s: %s", c.nwk, addr, err)
}

// redialPauseLoop is called to wait/loop on dialing
// a problematic address
func (c *clusterClient) redialPauseLoop(addr string) {
	for {
		time.Sleep(3 * time.Second)
		err := c.dial(c.nwk, addr, false)
		if err == nil {
			break
		}
		log.Printf("synapse cluster: error dialing %s: %s", addr, err)
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

			// errors for which we do nothing
			case syscall.EALREADY:
				return

			// errors for which we permanently drop the remote
			case syscall.EACCES, syscall.EADDRINUSE, syscall.EADDRNOTAVAIL,
				syscall.EAFNOSUPPORT, syscall.EBADMSG, syscall.EDQUOT, syscall.EFAULT,
				syscall.EOPNOTSUPP, syscall.ESOCKTNOSUPPORT:
				if c.remove(v) {
					log.Printf("synapse cluster: permanently dropping remote @ %s: %s", v.conn.RemoteAddr(), err)
				}

			// sleep 3; redial
			case syscall.EHOSTDOWN, syscall.EHOSTUNREACH:
				if c.remove(v) {
					go func(ad net.Addr) {
						for {
							log.Printf("synapse cluster: cannot reach host @ %s; redial in 3 seconds", ad)
							time.Sleep(3 * time.Second)
							err := c.dial(ad.Network(), ad.String(), false)
							if err != syscall.EHOSTDOWN && err != syscall.EHOSTUNREACH {
								log.Printf("synapse cluster: dropping remote @ %s: %s", ad, err)
								break
							}
						}
					}(v.conn.RemoteAddr())
				}

			// immediate redial
			case syscall.EPIPE, syscall.ESHUTDOWN, syscall.ESTALE:
				go c.redial(v)

			}
		}
	}
}
