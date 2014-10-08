package synapse

import (
	"errors"
	"github.com/philhofer/msgp/enc"
	"log"
	"net"
	"sync"
	"sync/atomic"
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

	// Status returns the current
	// connection status of the cluster.
	// (Calls to Status will temporarily
	// lock access to the client list, so
	// users should avoid calling it frequently.)
	Status() *ClusterStatus

	nope() // lol
}

// DialCluster creates a client out of multiple servers. It tries
// to maintain connections to the servers by re-dialing them on
// connection failures and removing problematic nodes from the
// connection pool. You must supply at least one remote address.
func DialCluster(network string, addrs ...string) (ClusterClient, error) {
	if len(addrs) < 1 {
		return nil, ErrNoClients
	}

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
				log.Printf("synapse cluster: error dialing %s %s: %s", c.nwk, addr, err)
				c.handleDialError(addr, err)
				errs[idx] = err
				return
			}
			// we can take advantage of resolved DNS, etc.
			c.remotes[i] = conn.RemoteAddr().String()
			cl, err := newClient(conn, 1000)
			if err != nil {
				log.Printf("synapse cluster: error establish sync with %s %s: %s", c.nwk, c.remotes[i], err)
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
		} else if first == -1 && err != nil {
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

// ClusterStatus is the
// connection state of
// a client connected to
// multiple endpoints
type ClusterStatus struct {
	Network      string   // "tcp", "unix", etc.
	Connected    []string // addresses of connected servers
	Disconnected []string // addresses of disconnected servers
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
	return c.dial(c.nwk, addr)
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

func (c *clusterClient) Status() *ClusterStatus {
	cc := &ClusterStatus{
		Network: c.nwk,
	}

	// this is the more "expensive"
	// lock to acquire, so we'll acquire
	// it first and then do heavy lifting
	// in the second (cheap) lock
	c.Lock()
	c.remoteLock.Lock()
	cc.Connected = make([]string, len(c.clients))
	for i, cl := range c.clients {
		cc.Connected[i] = cl.conn.RemoteAddr().String()
	}
	c.Unlock()

	cc.Disconnected = make([]string, 0, len(c.remotes))

	for _, rem := range c.remotes {
		dialed := false
		for _, cn := range cc.Connected {
			if rem == cn {
				dialed = true
				break
			}
		}
		if !dialed {
			cc.Disconnected = append(cc.Disconnected, rem)
		}
	}
	c.remoteLock.Unlock()

	return cc
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

// dial; add client and remote
func (c *clusterClient) dial(inet, addr string) error {
	conn, err := net.Dial(inet, addr)
	if err != nil {
		return err
	}
	c.remoteLock.Lock()
	found := false
	for _, rem := range c.remotes {
		if rem == conn.RemoteAddr().String() {
			found = true
			break
		}
	}
	if !found {
		c.remotes = append(c.remotes, conn.RemoteAddr().String())
	}
	c.remoteLock.Unlock()
	cl, err := newClient(conn, 1000)
	if err != nil {
		return err
	}
	c.add(cl)
	return nil
}

func (c *clusterClient) addAnother() error {
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

func (c *clusterClient) nope() {}
