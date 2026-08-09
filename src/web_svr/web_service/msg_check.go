package web_service

import (
	"github.com/Iori372552686/GoOne/lib/util/sensitive_words"
	define "github.com/Iori372552686/GoOne/src/web_svr/common"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

// MsgSecCheck processes a message security check request.
func MsgSecCheck(req *define.MsgSecCheckReq) *g1_protocol.Ret {
	rsp := &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_OK}
	if req == nil {
		rsp.Code = g1_protocol.ErrorCode_ERR_ARGV
		rsp.Msg = "msg check req is nil"
		return rsp
	}

	// 检查敏感字
	hasSensitiveWord, _ := sensitive_words.ChangeSensitiveWords(req.MsgContent)
	if hasSensitiveWord {
		rsp.Code = g1_protocol.ErrorCode_ERR_SENSITIVE_WORD
		rsp.Msg = "sensitive words found in msg content"
		return rsp
	}

	return rsp
}
