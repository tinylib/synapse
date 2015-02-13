package synapse

import (
	"sync"
	"testing"
)

func TestWaiterMap(t *testing.T) {
	mp := &wMap{}

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

	vals[200].done.Lock()
	mp.insert(&vals[200])

	mp.flush(nil)

	if mp.length() != 0 {
		t.Errorf("expected map length to be 0 after flush(); found %d", mp.length())
	}

	for i := range vals {
		vals[i].done.Lock()
		mp.insert(&vals[i])
	}

	l = mp.length()
	if l != 1000 {
		t.Errorf("expected 1000 elements after re-insertion; found %d", l)
	}

	mp.reap()

	l = mp.length()
	if l != 1000 {
		t.Errorf("expected 1000 elements after first reap(); found %d", l)
	}

	mp.reap()

	l = mp.length()
	if l != 0 {
		t.Errorf("expected 0 elements after second reap(); found %d", l)
	}

	for i := range vals {
		vals[i].done.Lock()
		vals[i].reap = false
		mp.insert(&vals[i])
	}

	// non-sequential removal
	for i := range vals {
		x := &vals[len(vals)-i-1]
		v := mp.remove(x.seq)
		if v == nil {
			t.Errorf("index %d: no value returned", len(vals)-i-1)
		}
		if v != x {
			t.Errorf("expected %v out; got %v", x, v)
		}
	}
}

func seqInsert(mp *wMap, b *testing.B, num int) {
	vals := make([]waiter, num)
	for i := range vals {
		vals[i].seq = uint64(i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := range vals {
			mp.insert(&vals[j])
		}
		for j := range vals {
			mp.remove(uint64(j))
		}
	}
}

func stdInsert(mp map[uint64]*waiter, b *testing.B, num int) {
	vals := make([]waiter, num)
	for i := range vals {
		vals[i].seq = uint64(i)
	}
	// the mutex is just for symmetry
	// with the other map implementation,
	// as it does one lock per insert/delete
	var m sync.Mutex
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := range vals {
			m.Lock()
			mp[vals[j].seq] = &vals[j]
			m.Unlock()
		}
		for j := range vals {
			m.Lock()
			if mp[vals[j].seq] != nil {
				delete(mp, vals[j].seq)
			}
			m.Unlock()
		}
	}
}

func BenchmarkMapInsertDelete(b *testing.B) {
	var mp wMap
	w := &waiter{}
	w.seq = 39082134
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mp.insert(w)
		mp.remove(w.seq)
	}
}

func BenchmarkStdMapInsertDelete(b *testing.B) {
	mp := make(map[uint64]*waiter)
	w := &waiter{}
	w.seq = 39082134
	b.ReportAllocs()
	b.ResetTimer()
	var m sync.Mutex
	for i := 0; i < b.N; i++ {
		m.Lock()
		mp[w.seq] = w
		m.Unlock()
		m.Lock()
		w = mp[w.seq]
		delete(mp, w.seq)
		m.Unlock()
	}
}

func Benchmark1000Sequential(b *testing.B) {
	var mp wMap
	seqInsert(&mp, b, 1000)
}

func Benchmark500Sequential(b *testing.B) {
	var mp wMap
	seqInsert(&mp, b, 500)
}

func Benchmark1000StdSequential(b *testing.B) {
	mp := make(map[uint64]*waiter)
	stdInsert(mp, b, 1000)
}

func Benchmark500StdSequential(b *testing.B) {
	mp := make(map[uint64]*waiter)
	stdInsert(mp, b, 500)
}
