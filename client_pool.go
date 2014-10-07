package synapse

import (
	"errors"
	"github.com/philhofer/msgp/enc"
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
	Add(network, addr string) error
}

func DialCluster(network string, addrs ...string) (ClusterClient, error) {
	c := new(clusterClient)
	errs := make(chan error, len(addrs))
	wg := new(sync.WaitGroup)

	for _, addr := range addrs {
		wg.Add(1)
		go func(remote string) {
			conn, err := net.Dial(network, remote)
			if err != nil {
				errs <- err
				return
			}
			nc, err := newClient(conn, 1000)
			if err != nil {
				errs <- err
				return
			}
			c.add(nc)
			errs <- nil
			wg.Done()
		}(addr)
	}

	// we should wait for
	// every dial to finish,
	// b/c we may have to close
	// the client, in which case
	// we don't want dials
	// racing on close
	wg.Wait()
	close(errs)

	for err := range errs {
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
// if an error warrants removing a client
// from the cluster's client list (bad pipe, etc.)
func (c *clusterClient) handleErr(v *client, err error) {
	if err == nil {
		return
	}

	// TODO: figure out possible network-related
	// errors and remove the node if it is
	// behaving poorly. For example:
	if neterr, ok := err.(net.Error); ok {
		if !neterr.Temporary() {
			c.remove(v)
			v.Close()
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

// remove a client from the list (threadsafe); no-op if it isn't there
func (c *clusterClient) remove(n *client) {
	c.Lock()
	for i, v := range c.clients {
		if v == n {
			l := len(c.clients)
			c.clients, c.clients[i], c.clients[l-1] = c.clients[:l-1], c.clients[l-1], nil
			break
		}
	}
	c.Unlock()
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
