package synapse

import (
	"net"
	"sort"
	"sync"
	"time"
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
	name, net, addr string
	distance        int32
}

func (s *Service) eqaddr(g *Service) bool {
	return s.net == g.net && s.addr == g.addr
}

func (s *Service) String() string {
	return s.name + "@" + s.net + ":" + s.addr
}

func (s *Service) Addr() (net, addr string) {
	net, addr = s.net, s.addr
	return
}

func (s *Service) Connect(timeout time.Duration) (*Client, error) {
	conn, err := net.Dial(s.net, s.addr)
	if err != nil {
		uncache(s)
		return nil, err
	}
	return NewClient(conn, timeout)
}

type serviceList []*Service

func (s serviceList) Len() int           { return len(s) }
func (s serviceList) Less(i, j int) bool { return s[i].distance < s[j].distance }
func (s serviceList) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func addSvc(list []*Service, s *Service) []*Service {
	if len(list) == 0 {
		return []*Service{s}
	}
	for _, sv := range list {
		if sv.eqaddr(s) {
			if s.distance < sv.distance {
				sv.distance = s.distance
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

var svcCache struct {
	sync.Mutex
	tab serviceTable
}

func init() {
	svcCache.tab = make(serviceTable)
}

func cache(s *Service) {
	svcCache.Lock()
	svcCache.tab[s.name] = addSvc(svcCache.tab[s.name], s)
	svcCache.Unlock()
}

func cachelist(l serviceList) {
	svcCache.Lock()
	for _, sv := range l {
		svcCache.tab[sv.name] = addSvc(svcCache.tab[sv.name], sv)
	}
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
