package synapse

import (
	"testing"
)

type testval struct {
	seq uint64
	w   *waiter
}

func TestWaiterMap(t *testing.T) {
	mp := &waiterMap{}

	q := mp.remove(4309821)
	if q != nil {
		t.Error("remove() should return 'nil' from an empty map")
	}

	vals := make([]testval, 1000)
	for i := range vals {
		vals[i].seq = uint64(i) * 17
		vals[i].w = &waiter{}
	}

	for _, v := range vals {
		mp.insert(v.seq, v.w)
	}

	for _, x := range vals {
		q := mp.remove(x.seq)
		if q != x.w {
			t.Errorf("expected %v; got %v", x.w, q)
		}
	}
}
