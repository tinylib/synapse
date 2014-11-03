package synapse

import (
	"github.com/philhofer/msgp/msgp"
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
func JSPipe(w io.Writer) msgp.Unmarshaler { return jsp{Writer: w} }

// thin wrapper for io.Writer to use
// as a MsgDecoder
type jsp struct {
	io.Writer
}

func (j jsp) UnmarshalMsg(b []byte) ([]byte, error) {
	return msgp.UnmarshalAsJSON(j, b)
}
