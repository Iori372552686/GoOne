package plug

import (
	"bytes"
	"github.com/bytedance/sonic/encoder"
	"net/http"
)

const DingDingFatalHookAddr = "https://oapi.dingtalk.com/robot/send?access_token=d92402471615f696534f8eb2ca3d8f7d79e1b5393e5d091df777a9ac1cf213e5"

func UploadFatalToDingHook(msgbody string) {
	body := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": "logger",
			"text":  msgbody,
		},
	}

	var data = bytes.NewBuffer(nil)
	encoder.NewStreamEncoder(data).Encode(body)
	client := http.Client{}
	client.Post(DingDingFatalHookAddr, "application/json", data)
}
