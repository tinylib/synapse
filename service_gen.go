package synapse

// NOTE: THIS FILE WAS PRODUCED BY THE
// MSGP CODE GENERATION TOOL (github.com/tinylib/msgp)
// DO NOT EDIT

import (
	"github.com/tinylib/msgp/msgp"
)

// MarshalMsg implements msgp.Marshaler
func (z *Service) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 4
	// string "name"
	o = append(o, 0x84, 0xa4, 0x6e, 0x61, 0x6d, 0x65)
	o = msgp.AppendString(o, z.name)
	// string "net"
	o = append(o, 0xa3, 0x6e, 0x65, 0x74)
	o = msgp.AppendString(o, z.net)
	// string "addr"
	o = append(o, 0xa4, 0x61, 0x64, 0x64, 0x72)
	o = msgp.AppendString(o, z.addr)
	// string "distance"
	o = append(o, 0xa8, 0x64, 0x69, 0x73, 0x74, 0x61, 0x6e, 0x63, 0x65)
	o = msgp.AppendInt32(o, z.distance)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *Service) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var isz uint32
	isz, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for isz > 0 {
		isz--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "name":
			z.name, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "net":
			z.net, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "addr":
			z.addr, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "distance":
			z.distance, bts, err = msgp.ReadInt32Bytes(bts)
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

func (z *Service) Msgsize() (s int) {
	s = 1 + 5 + msgp.StringPrefixSize + len(z.name) + 4 + msgp.StringPrefixSize + len(z.net) + 5 + msgp.StringPrefixSize + len(z.addr) + 9 + msgp.Int32Size
	return
}

// MarshalMsg implements msgp.Marshaler
func (z serviceList) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	o = msgp.AppendArrayHeader(o, uint32(len(z)))
	for xvk := range z {
		if z[xvk] == nil {
			o = msgp.AppendNil(o)
		} else {
			o, err = z[xvk].MarshalMsg(o)
			if err != nil {
				return
			}
		}
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *serviceList) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var xsz uint32
	xsz, bts, err = msgp.ReadArrayHeaderBytes(bts)
	if err != nil {
		return
	}
	if cap((*z)) >= int(xsz) {
		(*z) = (*z)[:xsz]
	} else {
		(*z) = make(serviceList, xsz)
	}
	for bzg := range *z {
		if msgp.IsNil(bts) {
			bts, err = msgp.ReadNilBytes(bts)
			if err != nil {
				return
			}
			(*z)[bzg] = nil
		} else {
			if (*z)[bzg] == nil {
				(*z)[bzg] = new(Service)
			}
			bts, err = (*z)[bzg].UnmarshalMsg(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

func (z serviceList) Msgsize() (s int) {
	s = msgp.ArrayHeaderSize
	for bai := range z {
		if z[bai] == nil {
			s += msgp.NilSize
		} else {
			s += z[bai].Msgsize()
		}
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z serviceTable) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	o = msgp.AppendMapHeader(o, uint32(len(z)))
	for cmr, ajw := range z {
		o = msgp.AppendString(o, cmr)
		o = msgp.AppendArrayHeader(o, uint32(len(ajw)))
		for wht := range ajw {
			if ajw[wht] == nil {
				o = msgp.AppendNil(o)
			} else {
				o, err = ajw[wht].MarshalMsg(o)
				if err != nil {
					return
				}
			}
		}
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *serviceTable) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var msz uint32
	msz, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	if (*z) == nil && msz > 0 {
		(*z) = make(serviceTable, msz)
	} else if len((*z)) > 0 {
		for key, _ := range *z {
			delete((*z), key)
		}
	}
	for msz > 0 {
		var hct string
		var cua serviceList
		msz--
		hct, bts, err = msgp.ReadStringBytes(bts)
		if err != nil {
			return
		}
		var xsz uint32
		xsz, bts, err = msgp.ReadArrayHeaderBytes(bts)
		if err != nil {
			return
		}
		if cap(cua) >= int(xsz) {
			cua = cua[:xsz]
		} else {
			cua = make(serviceList, xsz)
		}
		for xhx := range cua {
			if msgp.IsNil(bts) {
				bts, err = msgp.ReadNilBytes(bts)
				if err != nil {
					return
				}
				cua[xhx] = nil
			} else {
				if cua[xhx] == nil {
					cua[xhx] = new(Service)
				}
				bts, err = cua[xhx].UnmarshalMsg(bts)
				if err != nil {
					return
				}
			}
		}
		(*z)[hct] = cua
	}
	o = bts
	return
}

func (z serviceTable) Msgsize() (s int) {
	s = msgp.MapHeaderSize
	if z != nil {
		for lqf, daf := range z {
			_ = daf
			s += msgp.StringPrefixSize + len(lqf) + msgp.ArrayHeaderSize
			for pks := range daf {
				if daf[pks] == nil {
					s += msgp.NilSize
				} else {
					s += daf[pks].Msgsize()
				}
			}
		}
	}
	return
}
