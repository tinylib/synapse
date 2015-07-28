package synapse

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"sort"
	"sync"
)

//go:generate msgp -unexported -io=false

// Nearest finds the "nearest" service
// with the given name.
func Nearest(service string) (s *Service) {
	svcCache.Lock()
	l := svcCache.tab[service]
	if len(l) > 0 {
		s = l[0]
	}
	svcCache.Unlock()
	return
}

// Services lists all of the known
// services endpoints for a given
// service name.
func Services(name string) []*Service {
	svcCache.Lock()
	l := svcCache.tab[name]
	if len(l) == 0 {
		svcCache.Unlock()
		return nil
	}
	dup := make([]*Service, len(l))
	copy(dup, l)
	svcCache.Unlock()
	return dup
}

// Service represents a unique address
// associated with a service.
type Service struct {
	host            uint64 // host id
	name, net, addr string // address
	dist            int32  // distance
}

func (s *Service) HostID() uint64 { return s.host }

func (s *Service) Name() string { return s.name }

func (s *Service) eqaddr(g *Service) bool {
	return s.net == g.net && s.addr == g.addr
}

// String returns {Name}#{HostID}@{net}:{addr}
// e.g. echo#9081234973@tcp:localhost:7000
func (s *Service) String() string {
	return fmt.Sprintf("%s#%d@%s:%s", s.name, s.host, s.net, s.addr)
}

func (s *Service) Addr() (net, addr string) {
	net, addr = s.net, s.addr
	return
}

type serviceList []*Service

func (s serviceList) Len() int { return len(s) }
func (s serviceList) Less(i, j int) bool {

	// for os-local addresses, prefer unix sockets
	// over loopback tcp
	if s[i].dist == 0 && s[i].dist == s[j].dist {
		if s[i].net == "unix" && s[j].net != "unix" {
			return true
		}
	}

	return s[i].dist < s[j].dist
}

func (s serviceList) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

func addSvc(list []*Service, s *Service) []*Service {
	if len(list) == 0 {
		return []*Service{s}
	}
	for _, sv := range list {
		if sv.eqaddr(s) {
			if s.dist < sv.dist {
				sv.dist = s.dist
			}
			return list
		}
	}
	out := append(list, s)
	sort.Sort(serviceList(out))
	return out
}

func removeSvc(list []*Service, s *Service) []*Service {
	ll := len(list)
	if ll == 0 {
		return nil
	}
	for i, sv := range list {
		if sv.eqaddr(s) {
			list[i], list[ll-1], list = list[ll-1], nil, list[:ll-1]
			sort.Sort(serviceList(list))
			return list
		}
	}
	return list
}

type serviceTable map[string]serviceList

// svcCache stores all of the
// known services to this hostid.
//
var svcCache struct {
	sync.Mutex
	tab serviceTable
}

// host id
var hostid uint64

func HostID() uint64 { return hostid }

func init() {
	svcCache.tab = make(serviceTable)

	var buf [8]byte
	rand.Read(buf[:])
	hostid = binary.LittleEndian.Uint64(buf[:])
}

func isRoutable(s *Service) bool {
	// unix sockets, etc. can
	// be connected to on the
	// same machine
	if s.dist == 0 {
		return true
	}

	switch s.net {
	case "tcp", "tcp6", "tcp4":
		a, err := net.ResolveTCPAddr(s.net, s.addr)
		if err != nil {
			errorf("couldn't resolve tcp addr %s: %s", s.addr, err)
			return false
		}
		// TODO: link-local addresses
		return a.IP.IsGlobalUnicast()
	default:
		return false
	}
}

func cache(s *Service) {
	svcCache.Lock()
	svcCache.tab[s.name] = addSvc(svcCache.tab[s.name], s)
	svcCache.Unlock()
}

func uncache(s *Service) {
	svcCache.Lock()
	svcCache.tab[s.name] = removeSvc(svcCache.tab[s.name], s)
	svcCache.Unlock()
}

func svclistbytes() []byte {
	svcCache.Lock()
	data, _ := svcCache.tab.MarshalMsg(nil)
	svcCache.Unlock()
	return data
}
