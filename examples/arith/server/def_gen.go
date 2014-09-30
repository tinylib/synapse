package main

// NOTE: THIS FILE WAS PRODUCED BY THE
// MSGP CODE GENERATION TOOL (github.com/philhofer/msgp)
// DO NOT EDIT

import (
	"github.com/philhofer/msgp/enc"
	"io"
	"bytes"
)

func (z *Num) Marshal() ([]byte, error) {
	var buf bytes.Buffer
	_, err := z.EncodeMsg(&buf)
	return buf.Bytes(), err
}

func (z *Num) Unmarshal(b []byte) error {
	_, err := z.DecodeMsg(bytes.NewReader(b))
	return err
}

func (z *Num) EncodeMsg(w io.Writer) (n int, err error) {
	en := enc.NewEncoder(w)
	return z.EncodeTo(en)
}

func (z *Num) EncodeTo(en *enc.MsgWriter) (n int, err error) {
	var nn int
	_ = nn

	if z == nil {
		nn, err = en.WriteNil()
		n += nn
		if err != nil {
			return
		}
	} else {

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

	}

	return
}

func (z *Num) DecodeMsg(r io.Reader) (n int, err error) {
	dc := enc.NewDecoder(r)
	n, err = z.DecodeFrom(dc)
	enc.Done(dc)
	return
}

func (z *Num) DecodeFrom(dc *enc.MsgReader) (n int, err error) {
	var sz uint32
	var nn int
	var field []byte
	_ = sz
	_ = nn
	_ = field

	if dc.IsNil() {
		nn, err = dc.ReadNil()
		n += nn
		if err != nil {
			return
		}
		z = nil
	} else {
		if z == nil {
			z = new(Num)
		}

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

	}

	return
}

