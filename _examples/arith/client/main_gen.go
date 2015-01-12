package main

// NOTE: THIS FILE WAS PRODUCED BY THE
// MSGP CODE GENERATION TOOL (github.com/tinylib/msgp)
// DO NOT EDIT

import (
	"github.com/tinylib/msgp/msgp"
)

// MarshalMsg implements the msgp.Marshaler interface
func (z *Num) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())

	o = msgp.AppendMapHeader(o, 1)

	o = msgp.AppendString(o, "val")

	o = msgp.AppendFloat64(o, z.Value)

	return
}

// UnmarshalMsg unmarshals a Num from MessagePack, returning any extra bytes
// and any errors encountered
func (z *Num) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field

	var isz uint32
	isz, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for xplz := uint32(0); xplz < isz; xplz++ {
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {

		case "val":

			z.Value, bts, err = msgp.ReadFloat64Bytes(bts)

			if err != nil {
				return
			}

		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				return
			}
		}
	}

	o = bts
	return
}

// Msgsize implements the msgp.Sizer interface
func (z *Num) Msgsize() (s int) {

	s += msgp.MapHeaderSize
	s += msgp.StringPrefixSize + 3

	s += msgp.Float64Size

	return
}
