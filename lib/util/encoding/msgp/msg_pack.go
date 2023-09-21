// Package msgp defines the protobuf codec. Importing this package will
// register the codec.
package msgp

import (
	"GoOne/lib/util/encoding"
	"github.com/vmihailenco/msgpack"
)

// Name is the name registered for the msgpack compressor.
const Name = "msgpack"

func init() {
	encoding.RegisterCodec(codec{})
}

// codec is a Codec implementation with protobuf. It is the default codec for Transport.
type codec struct{}

func (codec) Marshal(v interface{}) ([]byte, error) {
	return msgpack.Marshal(v)
}

func (codec) Unmarshal(data []byte, v interface{}) error {
	return msgpack.Unmarshal(data, v)
}

func (codec) Name() string {
	return Name
}
