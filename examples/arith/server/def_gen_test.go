package main

// NOTE: THIS FILE WAS PRODUCED BY THE
// MSGP CODE GENERATION TOOL (github.com/philhofer/msgp)
// DO NOT EDIT

import (
	"testing"
	"bytes"
	"github.com/philhofer/msgp/enc"
)

func TestEncodeDecodeNum(t *testing.T) {
	t.Parallel()
	v := new(Num)
	var buf bytes.Buffer
	n, _ := v.EncodeMsg(&buf)

	vn := new(Num)
	nr, err := vn.DecodeMsg(&buf)
	if err != nil {
		t.Error(err)
	}

	if nr != n {
		t.Errorf("Wrote %d bytes; read %d bytes", n, nr)
	}

	buf.Reset()
	v.EncodeMsg(&buf)

	_, err = enc.NewDecoder(&buf).Skip()
	if err != nil {
		t.Error(err)
	}

	buf.Reset()
	v.EncodeMsg(&buf)

	ls := new(Num)
	var left []byte
	left, err = ls.UnmarshalMsg(buf.Bytes())
	if err != nil {
		t.Error(err)
	}
	if len(left) > 0 {
		t.Error("bytes left over...")
	}

	left, err = enc.Skip(buf.Bytes())
	if err != nil {
		t.Error(err)
	}
	if len(left) > 0 {
		t.Errorf("bytes left over after skip: %q", left)
	}
}

func BenchmarkWriteNum(b *testing.B) {
	v := new(Num)
	var buf bytes.Buffer
	en := enc.NewEncoder(&buf)
	v.EncodeTo(en)
	b.ReportAllocs()
	b.SetBytes(int64(buf.Len()))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		v.EncodeTo(en)
	}
}

func BenchmarkReadNum(b *testing.B) {
	v := new(Num)
	var buf bytes.Buffer
	v.EncodeMsg(&buf)
	rd := bytes.NewReader(buf.Bytes())
	dc := enc.NewDecoder(rd)
	b.ReportAllocs()
	b.SetBytes(int64(buf.Len()))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rd.Seek(0, 0)
		_, err := v.DecodeFrom(dc)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnmarshalNum(b *testing.B) {
	v := new(Num)
	var buf bytes.Buffer
	v.EncodeMsg(&buf)
	b.ReportAllocs()
	b.SetBytes(int64(buf.Len()))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := v.UnmarshalMsg(buf.Bytes())
		if err != nil {
			b.Fatal(err)
		}
	}
}
