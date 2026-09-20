package grpcapi

import (
	"encoding/json"
	"fmt"

	"google.golang.org/grpc/encoding"
)

const jsonCodecName = "json"

type jsonCodec struct{}

func (jsonCodec) Marshal(v interface{}) ([]byte, error)   { return json.Marshal(v) }
func (jsonCodec) Unmarshal(data []byte, v interface{}) error { return json.Unmarshal(data, v) }
func (jsonCodec) Name() string { return jsonCodecName }

func RegisterJSONCodec() {
	encoding.RegisterCodec(jsonCodec{})
}

func init() {
	RegisterJSONCodec()
}

func statusError(msg string) error {
	return fmt.Errorf("rpc error: %s", msg)
}
