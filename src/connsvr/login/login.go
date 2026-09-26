package login

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/util/convert"
	"github.com/Iori372552686/GoOne/module/conf"
	"github.com/Iori372552686/GoOne/src/connsvr/globals"
)

type accountProfileResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		AccountID   int64                  `json:"account_id"`
		GameProfile map[string]interface{} `json:"game_profile"`
	} `json:"data"`
}

// OnCheckAuthByAccSvr 校验账号并返回真实 uid。
//
// 两种模式：
//   - dev（base_cfg.runtime.env_mode == "dev"）：跳过账号服，account 为纯数字时
//     以该数字为 uid，否则返回 (true, 0)——调用方回退使用客户端自报 uid（tester 等
//     外部预分配 uid 的接入方依赖此回退）。
//   - 非 dev：GET base_cfg.dependencies.rest_api_config[default]，从 game_profile.uid
//     解析真实 uid；任何失败返回 (false, 0)，调用方应丢弃该首包。
func OnCheckAuthByAccSvr(accId string, token string, serverid uint32, loginType string) (bool, uint64) {
	accountID := strings.TrimSpace(accId)
	if accountID == "" {
		logger.Errorf("OnCheckAuthByAccSvr missing account_id, channel_id=%d login_type=%s", serverid, loginType)
		return false, 0
	}

	if conf.Get("base_cfg.runtime.env_mode").String() == "dev" {
		uid := convert.StrToInt(accountID)
		logger.Warningf("[DEV MODE] skip account server auth, account=%s channel=%d uid=%d", accountID, serverid, uid)
		return true, uint64(uid)
	}

	restIns := globals.RestMgr.GetRestIns()
	if restIns == nil {
		logger.Errorf("OnCheckAuthByAccSvr default rest api instance not found, account_id=%s", accountID)
		return false, 0
	}

	query := map[string]string{
		"account_id": accountID,
		"game_id":    conf.Get("connsvr.runtime.game_id").String(),
	}
	headers := map[string]string{
		"Authorization": token,
		"X-Account-Id":  accountID,
		"Content-Type":  "application/json",
	}

	rspBody, err := restIns.SignedGetWithHeaders(context.Background(), 0, query, headers)
	if err != nil {
		logger.Errorf("OnCheckAuthByAccSvr request profile failed, account_id=%s channel_id=%d login_type=%s err=%v", accountID, serverid, loginType, err)
		return false, 0
	}

	var result accountProfileResponse
	if err = json.Unmarshal(rspBody, &result); err != nil {
		logger.Errorf("OnCheckAuthByAccSvr decode profile failed, account_id=%s err=%v body=%s", accountID, err, string(rspBody))
		return false, 0
	}

	logger.Infof("OnCheckAuthByAccSvr get profile success, account_id=%s profile=%+v", accountID, result.Data.GameProfile)
	if result.Code != 0 {
		logger.Errorf("OnCheckAuthByAccSvr profile returned non-zero code, account_id=%s code=%d msg=%s", accountID, result.Code, result.Msg)
		return false, 0
	}

	uid, ok := parseProfileUID(result.Data.GameProfile)
	if !ok || uid <= 0 {
		logger.Errorf("OnCheckAuthByAccSvr profile missing uid, account_id=%s game_profile=%+v", accountID, result.Data.GameProfile)
		return false, 0
	}

	return true, uint64(uid)
}

func parseProfileUID(gameProfile map[string]interface{}) (int64, bool) {
	if gameProfile == nil {
		return 0, false
	}
	value, ok := gameProfile["uid"]
	if !ok {
		return 0, false
	}
	switch v := value.(type) {
	case string:
		return int64(convert.StrToInt(v)), true
	case float64:
		return int64(v), true
	case int:
		return int64(v), true
	case int64:
		return v, true
	default:
		return 0, false
	}
}
