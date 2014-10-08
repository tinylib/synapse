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
	Add(addr string) error
}

func DialCluster(network string, addrs ...string) (ClusterClient, error) {
	c := &clusterClient{
		nwk:     network,
		remotes: addrs,
	}
	err := c.dialAll()
	if err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

// dialAll dials all of the known
// connections *if* there are zero
// clients in the current list. it
// holds the client list lock for
// the entirety of the dialing period.
func (c *clusterClient) dialAll() error {
	c.Lock()

	// we could have raced
	// on acquiring the lock,
	// but we only want one process
	// trying to re-dial connections
	if len(c.clients) > 0 {
		c.Unlock()
		return nil
	}

	// temporary lock for growing
	// the client list
	addLock := new(sync.Mutex)

	c.remoteLock.Lock()

	if len(c.remotes) == 0 {
		c.remoteLock.Unlock()
		c.Unlock()
		return ErrNoClients
	}

	errs := make([]error, len(c.remotes))
	wg := new(sync.WaitGroup)
	wg.Add(len(c.remotes))

	for i, addr := range c.remotes {
		go func(idx int, addr string, wg *sync.WaitGroup) {
			defer wg.Done()
			conn, err := net.Dial(c.nwk, addr)
			if err != nil {
				errs[idx] = err
				return
			}
			cl, err := newClient(conn, 1000)
			if err != nil {
				errs[idx] = err
				return
			}
			addLock.Lock()
			c.clients = append(c.clients, cl)
			addLock.Unlock()
			return
		}(i, addr, wg)
	}

	wg.Wait()
	c.remoteLock.Unlock()
	c.Unlock()

	// we only return an
	// error if we are unable
	// to reach *any* of the
	// servers
	any := false
	first := -1
	for i, err := range errs {
		if err == nil && !any {
			any = true
		} else if first == -1 {
			first = i
		}
	}
	if !any {
		return errs[first]
	}
	return nil
}

type clusterClient struct {
	idx        uint64     // round robin counter
	sync.Mutex            // lock for connections
	clients    []*client  // connections
	nwk        string     // network type
	remotes    []string   // remote addr
	remoteLock sync.Mutex // remote addr list lock
}

func (c *clusterClient) Call(method string, in enc.MsgEncoder, out enc.MsgDecoder) error {
next:
	v := c.next()
	if v == nil {
		// if no clients are present,
		// we need to dial.
		err := c.dialAll()
		if err != nil {
			return err
		}
		goto next
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

func (c *clusterClient) Add(addr string) error {
	// dial the server first;
	// add the node to the remote
	// list only if we can actually
	// connect to the node.
	err := c.dial(c.nwk, addr)
	if err != nil {
		return err
	}
	c.remoteLock.Lock()
	c.remotes = append(c.remotes, addr)
	c.remoteLock.Unlock()
	return nil
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
	// don't race on removal
	if c.remove(n) {
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

func (c *clusterClient) addRandom() error {
	c.remoteLock.Lock()
	l := len(c.remotes)
	if l == 0 {
		c.remoteLock.Unlock()
		return ErrNoClients
	}
	this := atomic.AddUint64(&c.idx, 1)
	addr := c.remotes[this%uint64(l)]
	c.remoteLock.Unlock()

	return c.dial(c.nwk, addr)
}
