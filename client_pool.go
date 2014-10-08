package synapse

import (
	"errors"
	"github.com/philhofer/msgp/enc"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

var (
	ErrNoClients = errors.New("no clients available")
)

// ClusterClient is a client that
// connects to multiple servers
// and distributes requests among
// them.
type ClusterClient interface {
	Client

	// Add adds a remote server
	// to the list of server nodes
	Add(network, addr string) error
}

func DialCluster(network string, addrs ...string) (ClusterClient, error) {
	c := new(clusterClient)
	errs := make([]error, len(addrs))
	wg := new(sync.WaitGroup)

	wg.Add(len(addrs))
	for i, addr := range addrs {
		go func(remote string, idx int) {
			conn, err := net.Dial(network, remote)
			if err != nil {
				errs[idx] = err
				return
			}
			nc, err := newClient(conn, 1000)
			if err != nil {
				errs[idx] = err
				return
			}
			c.add(nc)
			errs[idx] = nil
			wg.Done()
		}(addr, i)
	}

	wg.Wait()

	for _, err := range errs {
		if err != nil {
			c.Close()
			return nil, err
		}
	}

	return c, nil
}

type clusterClient struct {
	sync.Mutex           // lock for connections
	idx        uint64    // round robin counter
	clients    []*client // connections
}

func (c *clusterClient) Call(method string, in enc.MsgEncoder, out enc.MsgDecoder) error {
	v := c.next()
	if v == nil {
		return ErrNoClients
	}
	err := v.Call(method, in, out)
	if err != nil {
		c.handleErr(v, err)
	}
	return err
}

func (c *clusterClient) Async(method string, in enc.MsgEncoder) (AsyncResponse, error) {
	v := c.next()
	if v == nil {
		return nil, ErrNoClients
	}
	asn, err := v.Async(method, in)
	if err != nil {
		c.handleErr(v, err)
	}
	return asn, err
}

func (c *clusterClient) Add(network, addr string) error {
	return c.dial(network, addr)
}

func (c *clusterClient) Close() error {
	wg := new(sync.WaitGroup)
	c.Lock()
	for _, v := range c.clients {
		wg.Add(1)
		go func(wg *sync.WaitGroup, cl *client) {
			cl.Close()
			wg.Done()
		}(wg, v)
	}
	c.clients = c.clients[:0]
	c.Unlock()
	return nil
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
							err := c.dial(ad.Network(), ad.String())
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

// acquire a client pointer via round-robin (threadsafe)
func (c *clusterClient) next() *client {
	i := atomic.AddUint64(&c.idx, 1)
	c.Lock()
	l := len(c.clients)
	if l == 0 {
		c.Unlock()
		return nil
	}
	o := c.clients[i%uint64(l)]
	c.Unlock()
	return o
}

// add a client to the list via append (threadsafe)
func (c *clusterClient) add(n *client) {
	c.Lock()
	c.clients = append(c.clients, n)
	c.Unlock()
}

// remove a client from the list (atomic); return
// whether or not is was removed so we can determine
// the winner of races to remove the client
func (c *clusterClient) remove(n *client) bool {
	found := false
	c.Lock()
	for i, v := range c.clients {
		if v == n {
			l := len(c.clients)
			c.clients, c.clients[i], c.clients[l-1] = c.clients[:l-1], c.clients[l-1], nil
			found = true
			break
		}
	}
	c.Unlock()
	return found
}

// redial idempotently re-dials a client.
// redial no-ops if the client isn't in the
// client list to begin with in order to prevent
// races on re-dialing.
func (c *clusterClient) redial(n *client) {
	// remove client; check
	// to see if other redial
	// processes beat us to
	// removal
	in := c.remove(n)

	// someone else
	// already removed
	// the client, and is
	// presumably re-dialing it
	if !in {
		return
	}

	n.Close()

	log.Printf("synapse cluster: re-dialing %s", n.conn.RemoteAddr())

	remote := n.conn.RemoteAddr()
	err := c.dial(remote.Network(), remote.String())
	if err != nil {
		log.Printf("synapse cluster: re-dialing node @ %s failed: %s", remote.String(), err)
	} else {
		log.Printf("synapse cluster: successfully re-connected to %s", n.conn.RemoteAddr())
	}
}

// dial; add client on success (threadsafe)
func (c *clusterClient) dial(inet, addr string) error {
	conn, err := net.Dial(inet, addr)
	if err != nil {
		return err
	}

	cl, err := newClient(conn, 1000)
	if err != nil {
		return err
	}
	c.add(cl)
	return nil
}
