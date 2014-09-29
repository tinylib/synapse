package synapse

// NOTE: THIS FILE WAS PRODUCED BY THE
// MSGP CODE GENERATION TOOL (github.com/philhofer/msgp)
// DO NOT EDIT

import (
	"bytes"
	"github.com/philhofer/msgp/enc"
	"io"
)

func (z *Arg1) Marshal() ([]byte, error) {
	var buf bytes.Buffer
	_, err := z.EncodeMsg(&buf)
	return buf.Bytes(), err
}

func (z *Arg1) Unmarshal(b []byte) error {
	_, err := z.DecodeMsg(bytes.NewReader(b))
	return err
}

func (z *Arg1) EncodeMsg(w io.Writer) (n int64, err error) {
	var nn int
	en := enc.NewEncoder(w)
	_ = nn
	_ = en

	if z == nil {
		nn, err = en.WriteNil()
		n += int64(nn)
		if err != nil {
			return
		}
	} else {

		nn, err = en.WriteMapHeader(1)
		n += int64(nn)
		if err != nil {
			return
		}

		nn, err = en.WriteString("value")
		n += int64(nn)
		if err != nil {
			return
		}

		nn, err = en.WriteString(z.Value)

		n += int64(nn)
		if err != nil {
			return
		}

	}

	return
}

func (z *Arg1) DecodeMsg(r io.Reader) (n int64, err error) {
	var sz uint32
	var nn int
	var field []byte
	dc := enc.NewDecoder(r)
	_ = sz
	_ = nn
	_ = field

	if dc.IsNil() {
		nn, err = dc.ReadNil()
		n += int64(nn)
		if err != nil {
			return
		}
		z = nil
	} else {
		if z == nil {
			z = new(Arg1)
		}

		var isz uint32
		isz, nn, err = dc.ReadMapHeader()
		n += int64(nn)
		if err != nil {
			return
		}
		for xplz := uint32(0); xplz < isz; xplz++ {
			field, nn, err = dc.ReadStringAsBytes(field)
			n += int64(nn)
			if err != nil {
				return
			}
			switch enc.UnsafeString(field) {

			case "value":

				z.Value, nn, err = dc.ReadString()

				n += int64(nn)
				if err != nil {
					return
				}

			default:
				nn, err = dc.Skip()
				n += int64(nn)
				if err != nil {
					return
				}
			}
		}

	}

	enc.Done(dc)
	return
}

func (z *Arg2) Marshal() ([]byte, error) {
	var buf bytes.Buffer
	_, err := z.EncodeMsg(&buf)
	return buf.Bytes(), err
}

func (z *Arg2) Unmarshal(b []byte) error {
	_, err := z.DecodeMsg(bytes.NewReader(b))
	return err
}

func (z *Arg2) EncodeMsg(w io.Writer) (n int64, err error) {
	var nn int
	en := enc.NewEncoder(w)
	_ = nn
	_ = en

	if z == nil {
		nn, err = en.WriteNil()
		n += int64(nn)
		if err != nil {
			return
		}
	} else {

		nn, err = en.WriteMapHeader(1)
		n += int64(nn)
		if err != nil {
			return
		}

		nn, err = en.WriteString("value")
		n += int64(nn)
		if err != nil {
			return
		}

		nn, err = en.WriteString(z.Value)

		n += int64(nn)
		if err != nil {
			return
		}

	}

	return
}

func (z *Arg2) DecodeMsg(r io.Reader) (n int64, err error) {
	var sz uint32
	var nn int
	var field []byte
	dc := enc.NewDecoder(r)
	_ = sz
	_ = nn
	_ = field

	if dc.IsNil() {
		nn, err = dc.ReadNil()
		n += int64(nn)
		if err != nil {
			return
		}
		z = nil
	} else {
		if z == nil {
			z = new(Arg2)
		}

		var isz uint32
		isz, nn, err = dc.ReadMapHeader()
		n += int64(nn)
		if err != nil {
			return
		}
		for xplz := uint32(0); xplz < isz; xplz++ {
			field, nn, err = dc.ReadStringAsBytes(field)
			n += int64(nn)
			if err != nil {
				return
			}
			switch enc.UnsafeString(field) {

			case "value":

				z.Value, nn, err = dc.ReadString()

				n += int64(nn)
				if err != nil {
					return
				}

			default:
				nn, err = dc.Skip()
				n += int64(nn)
				if err != nil {
					return
				}
			}
		}

	}

	enc.Done(dc)
	return
}
