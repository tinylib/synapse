package main

// NOTE: THIS FILE WAS PRODUCED BY THE
// MSGP CODE GENERATION TOOL (github.com/philhofer/msgp)
// DO NOT EDIT

import (
	"github.com/philhofer/msgp/enc"
	"io"
	"bytes"
)


// MarshalMsg marshals a Num into MessagePack
func (z *Num) MarshalMsg() ([]byte, error) {
	var buf bytes.Buffer
	_, err := z.EncodeMsg(&buf)
	return buf.Bytes(), err
}

// UnmarshalMsg unmarshals a Num from MessagePack, returning any extra bytes
// and any errors encountered
func (z *Num) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field

	var isz uint32
	isz, bts, err = enc.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for xplz := uint32(0); xplz < isz; xplz++ {
		field, bts, err = enc.ReadStringZC(bts)
		if err != nil {
			return
		}
		switch enc.UnsafeString(field) {

		case "val":

			z.Value, bts, err = enc.ReadFloat64Bytes(bts)

			if err != nil {
				return
			}

		default:
			bts, err = enc.Skip(bts)
			if err != nil {
				return
			}
		}
	}

	o = bts
	return
}

// EncodeMsg encodes a Num as MessagePack to the supplied io.Writer,
// returning the number of bytes written and any errors encountered
func (z *Num) EncodeMsg(w io.Writer) (n int, err error) {
	en := enc.NewEncoder(w)
	return z.EncodeTo(en)
}

// EncodeTo encodes a Num as MessagePack using the provided encoder,
// returning the number of bytes written and any errors encountered
func (z *Num) EncodeTo(en *enc.MsgWriter) (n int, err error) {
	var nn int
	_ = nn

	nn, err = en.WriteMapHeader(1)
	n += nn
	if err != nil {
		return
	}

	nn, err = en.WriteString("val")
	n += nn
	if err != nil {
		return
	}

	nn, err = en.WriteFloat64(z.Value)

	n += nn
	if err != nil {
		return
	}

	return
}

// DecodeMsg decodes MessagePack from the provided io.Reader into the Num,
// returning the number of bytes read and any errors encountered
func (z *Num) DecodeMsg(r io.Reader) (n int, err error) {
	dc := enc.NewDecoder(r)
	n, err = z.DecodeFrom(dc)
	enc.Done(dc)
	return
}

// DecodeFrom deocdes MessagePack from the provided decoder into the Num,
// returning the number of bytes read and any errors encountered.
func (z *Num) DecodeFrom(dc *enc.MsgReader) (n int, err error) {
	var nn int
	var field []byte
	_ = nn
	_ = field

	var isz uint32
	isz, nn, err = dc.ReadMapHeader()
	n += nn
	if err != nil {
		return
	}
	for xplz := uint32(0); xplz < isz; xplz++ {
		field, nn, err = dc.ReadStringAsBytes(field)
		n += nn
		if err != nil {
			return
		}
		switch enc.UnsafeString(field) {

		case "val":

			z.Value, nn, err = dc.ReadFloat64()

			n += nn
			if err != nil {
				return
			}

		default:
			nn, err = dc.Skip()
			n += nn
			if err != nil {
				return
			}
		}
	}

	return
}
