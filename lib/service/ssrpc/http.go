package ssrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/gin-gonic/gin"
	"github.com/golang/protobuf/jsonpb"
	proto1 "github.com/golang/protobuf/proto"
	"google.golang.org/protobuf/encoding/protojson"
	proto2 "google.golang.org/protobuf/proto"
)

type httpIContext struct {
	gin *gin.Context
}

var _ cmd_handler.IContext = (*httpIContext)(nil)

func (h *httpIContext) Uid() uint64         { return 0 }
func (h *httpIContext) Zone() uint32        { return 0 }
func (h *httpIContext) Rid() uint64         { return 0 }
func (h *httpIContext) OriSrcBusId() uint32 { return 0 }
func (h *httpIContext) Ip() uint32          { return 0 }
func (h *httpIContext) Flag() uint32        { return 0 }

func (h *httpIContext) Context() context.Context {
	if h == nil || h.gin == nil || h.gin.Request == nil {
		return context.Background()
	}
	return h.gin.Request.Context()
}

func (h *httpIContext) ParseMsg(data []byte, msg proto1.Message) error {
	return unmarshalJSONToProto(data, msg)
}

func (h *httpIContext) CallMsgBySvrType(uint32, g1_protocol.CMD, proto1.Message, proto1.Message) error {
	return errors.New("not supported in http context")
}
func (h *httpIContext) CallMsgByRouter(uint32, uint64, g1_protocol.CMD, proto1.Message, proto1.Message) error {
	return errors.New("not supported in http context")
}
func (h *httpIContext) CallOtherMsgBySvrType(uint32, uint64, uint64, uint32, g1_protocol.CMD, proto1.Message, proto1.Message) error {
	return errors.New("not supported in http context")
}
func (h *httpIContext) SendMsgBack(proto1.Message) {}
func (h *httpIContext) SendMsgByServerType(uint32, g1_protocol.CMD, proto1.Message) error {
	return errors.New("not supported in http context")
}
func (h *httpIContext) SendMsgByRouter(uint32, uint64, g1_protocol.CMD, proto1.Message) error {
	return errors.New("not supported in http context")
}

func (h *httpIContext) Errorf(format string, args ...interface{})   { logger.Errorf(format, args...) }
func (h *httpIContext) Warningf(format string, args ...interface{}) { logger.Warningf(format, args...) }
func (h *httpIContext) Infof(format string, args ...interface{})    { logger.Infof(format, args...) }
func (h *httpIContext) Debugf(format string, args ...interface{})   { logger.Debugf(format, args...) }

func unmarshalJSONToProto(data []byte, msg proto1.Message) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		data = []byte("{}")
	}
	// Prefer v2 protojson when available.
	if m2, ok := any(msg).(proto2.Message); ok {
		return protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(data, m2)
	}
	um := jsonpb.Unmarshaler{AllowUnknownFields: true}
	return um.Unmarshal(bytes.NewReader(data), msg)
}

func marshalProtoToJSONRaw(msg proto1.Message) (json.RawMessage, error) {
	if msg == nil {
		return nil, nil
	}
	if m2, ok := any(msg).(proto2.Message); ok {
		b, err := protojson.MarshalOptions{EmitUnpopulated: true, UseProtoNames: true}.Marshal(m2)
		if err != nil {
			return nil, err
		}
		return json.RawMessage(b), nil
	}
	var m jsonpb.Marshaler
	m.EmitDefaults = true
	m.OrigName = true
	s, err := m.MarshalToString(msg)
	if err != nil {
		return nil, err
	}
	return json.RawMessage([]byte(s)), nil
}

func httpErrorMessage(err error, code g1_protocol.ErrorCode) string {
	if err != nil {
		if msg := strings.TrimSpace(err.Error()); msg != "" && msg != "<nil>" {
			return msg
		}
	}
	return strings.TrimSpace(code.String())
}

// WrapHTTPGin returns a gin.HandlerFunc that decodes JSON->proto, runs middleware, and replies JSON.
func WrapHTTPGin(desc MethodDesc, mws []Middleware, newReq func() any, invoke func(ctx *Context, req any) (any, error)) gin.HandlerFunc {
	mws = prepareMW(mws, desc.UIDLock)
	h := buildHandler(mws, invoke) // pre-build chain once at init time
	return func(c *gin.Context) {
		if c == nil {
			return
		}
		data, _ := c.GetRawData()

		ic := &httpIContext{gin: c}
		ctx := WrapIContext(ic, desc.Cmd)
		ctx.SetTransport(TransportHTTP)
		ctx.SetHTTPRequest(c.Request, data)
		applyDesc(ctx, &desc)
		ctx.ApplyTimeout(effectiveMethodTimeout(desc.Timeout))
		defer ctx.Close()

		reqAny := newReq()
		req1, ok := reqAny.(proto1.Message)
		if !ok || req1 == nil {
			c.JSON(http.StatusOK, gin.H{"code": g1_protocol.ErrorCode_ERR_INTERNAL, "data": nil, "msg": "invalid req type"})
			return
		}
		if err := ic.ParseMsg(data, req1); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": g1_protocol.ErrorCode_ERR_MARSHAL, "data": nil, "msg": g1_protocol.ErrorCode_ERR_MARSHAL.String()})
			return
		}

		rsp, err := h(ctx, req1)
		if err != nil {
			code := ToErrorCode(err)
			c.JSON(http.StatusOK, gin.H{"code": code, "data": nil, "msg": httpErrorMessage(err, code)})
			return
		}
		if desc.OneWay || rsp == nil {
			c.JSON(http.StatusOK, gin.H{"code": g1_protocol.ErrorCode_ERR_OK, "data": nil, "msg": g1_protocol.ErrorCode_ERR_OK.String()})
			return
		}

		raw, mErr := marshalProtoToJSONRaw(rsp)
		if mErr != nil {
			c.JSON(http.StatusOK, gin.H{"code": g1_protocol.ErrorCode_ERR_MARSHAL, "data": nil, "msg": g1_protocol.ErrorCode_ERR_MARSHAL.String()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": g1_protocol.ErrorCode_ERR_OK, "data": raw, "msg": g1_protocol.ErrorCode_ERR_OK.String()})
	}
}
