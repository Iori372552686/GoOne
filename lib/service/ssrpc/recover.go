package ssrpc

import (
	"fmt"

	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/golang/protobuf/proto"
)

// Recover converts panics into an error so the wrapper can map it to an ErrorCode.
func Recover() Middleware {
	return func(next Handler) Handler {
		return func(ctx *Context, req proto.Message) (rsp proto.Message, err error) {
			defer func() {
				if r := recover(); r != nil {
					err = &Error{
						Code: g1_protocol.ErrorCode_ERR_INTERNAL,
						Msg:  "panic recovered",
						Err:  fmt.Errorf("%v", r),
					}
				}
			}()
			return next(ctx, req)
		}
	}
}


