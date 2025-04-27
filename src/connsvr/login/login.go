package login

import (
	"github.com/Iori372552686/GoOne/lib/util/convert"
	"github.com/Iori372552686/GoOne/src/connsvr/globals"
)

// php中台通讯消息体
type MiddleMsgDefault struct {
	Status bool `json:"status"`
	Msg    string
}

type MiddleMsgRole struct {
	MiddleMsgDefault
	Data MiddleRole `json:"data"`
}

// 中台角色信息
type MiddleRole struct {
	Id         int64 `json:"id"`      //rid
	UserId     int64 `json:"user_id"` //中台aid
	ChannelId  int64 `json:"chan_id"`
	Status     int8  `json:"status"`  //1正常 其他异常
	SiteId     int64 `json:"site_id"` //站点id
	Balance    int64 `json:"balance"` //余额
	Freeze     int64 `json:"freeze"`  //冻结金额
	UpdateTime int64 `json:"update_time"`
	CreateTime int64 `json:"create_time"`
}

// check auth by accsvr
func OnCheckAuthByAccSvr(accId string, token string, serverid uint32, loginType string) (bool, uint64) {
	header := &map[string]string{
		"Authorization": token,
		"Content-type":  "application/json",
	}

	body := &map[string]interface{}{
		"channel_id": serverid,
		"account_id": accId,
		"login_type": loginType,
	}

	rsqBody, err := globals.RestMgr.GetRestIns().SignPostV2(header, nil, body)
	if err != nil {
		return false, 0
	}

	var result MiddleMsgRole
	convert.JsonToStruct(convert.Bytes2str(rsqBody), &result)
	if result.Status == true {
		// if result["ret"] == "true" {
		return true, uint64(result.Data.Id)
	}

	return false, 0
}
