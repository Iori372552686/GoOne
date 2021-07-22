package feishuApi

import (
	"encoding/json"
	"net/url"

	"bian/src/common/util"
)

func SendNotifyToFeiShu(hookUrl, msg string) {
	if msg == "" || hookUrl == "" {
		return
	}

	// 创建请求data
	value := url.Values{}
	data := map[string]string{}
	value.Add("msg_type", "text")
	data["text"] = msg
	msgJson, err := json.Marshal(data)
	if err != nil {
		return
	}
	value.Add("content", string(msgJson))

	util.AuthHttpRequest("POST", value.Encode(), hookUrl)
}

func SendNotifyToFeiShuByRichText(hookUrl, content string) {
	if content == "" {
		return
	}

	// 创建请求data
	value := url.Values{}
	value.Add("msg_type", "post")
	value.Add("content", content)

	util.AuthHttpRequest("POST", value.Encode(), hookUrl)
}
