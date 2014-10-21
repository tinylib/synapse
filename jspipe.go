package synapse

import (
	"github.com/philhofer/msgp/enc"
	"io"
)

// JSPipe is a decoder that can be used
// to translate a messagepack object
// directly into JSON as it is being
// decoded.
//
// For example, you can trivially
// print the response to a request
// as JSON to stdout by writing
// something like the following:
//
//	res, _ := client.Async("thing", in)
//	res.Read(synapse.JSPipe(os.Stdout))
//
func JSPipe(w io.Writer) enc.MsgDecoder {
	return jsp{Writer: w}
}

// thin wrapper for io.Writer to use
// as a MsgDecoder
type jsp struct {
	io.Writer
}

func (j jsp) DecodeMsg(r io.Reader) (int, error) {
	n, err := enc.CopyToJSON(j, r)
	return int(n), err
}

func (j jsp) DecodeFrom(dc *enc.MsgReader) (int, error) {
	n, err := enc.DecodeToJSON(j, dc)
	return int(n), err
}

func (j jsp) UnmarshalMsg(b []byte) ([]byte, error) {
	return enc.UnmarshalAsJSON(j, b)
}
