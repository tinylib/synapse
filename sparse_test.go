package synapse

import (
	"testing"
)

func TestWaiterMap(t *testing.T) {
	mp := &waiterMap{}

	q := mp.remove(4309821)
	if q != nil {
		t.Error("remove() should return 'nil' from an empty map")
	}

	vals := make([]waiter, 1000)
	for i := range vals {
		vals[i].seq = uint64(i) * 17
	}

	for i := range vals {
		mp.insert(&vals[i])
	}

	if mp.length() != 1000 {
		t.Errorf("expected map to have 1000 elements; found %d", mp.length())
	}

	for i := range vals {
		q := mp.remove(vals[i].seq)
		if q != &vals[i] {
			t.Errorf("expected %v; got %v", &vals[i], q)
		}
	}

	l := mp.length()
	if l != 0 {
		t.Errorf("expected map to have 0 elements; found %d", l)
	}
}
