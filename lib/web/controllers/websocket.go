// Copyright 2013 Beego Samples authors
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

package controllers

import (
	`encoding/json`
	`log`
	`time`

	`GoOne/common`
	`GoOne/lib/logger`
	web `GoOne/lib/web/client`
)

// WebSocketController handles WebSocket requests.
type WebSocketController struct {
	baseController
}

// Get method handles GET requests for WebSocketController.
func (this *WebSocketController) Get() {
	// Safe check.
	accId := this.GetString("accId")
	if len(accId) == 0 {
		this.Redirect("/", 302)
		return
	}

	this.TplName = "websocket.html"
	this.Data["IsWebSocket"] = true
	this.Data["accId"] = accId
}

// Join method handles WebSocket requests for WebSocketController.
func (this *WebSocketController) Join() {
	ret := &common.JsonResult{}
	defer func() {
		byteArr, err := json.Marshal(ret)
		if err != nil { logger.Errorf(ret.Msg) }
		this.Ctx.WriteString(string(byteArr))
	}()

	accId := this.GetString("accId")
	appId, _ := this.GetUint32("appId")
	if len(accId) == 0 || appId == 0 {
		ret.Code = common.ParameterIllegal
		ret.Msg = "args err ！ "
		return
	}

	// todo auth
	// Upgrade from http request to WebSocket.
	web.WsInitClientPage(this.Ctx.ResponseWriter, this.Ctx.Request)
	ret.Code = common.OK
	return
}


// Heart
func (this *WebSocketController) Heart() {
	ret := &common.JsonResult{}
	defer func() {
		byteArr, err := json.Marshal(ret)
		if err != nil { logger.Errorf(ret.Msg) }
		this.Ctx.WriteString(string(byteArr))
	}()

	accId := this.GetString("accId")
	appId, _ := this.GetUint32("appId")
	if len(accId) == 0 || appId == 0 {
		ret.Code = common.ParameterIllegal
		ret.Msg = "args err ！ "
		return
	}

	currentTime := uint64(time.Now().Unix())
	log.Println("webSocket_Heart accId =", accId)
	client := web.GetUserClient(accId)
	if client == nil {
		ret.Code = common.NotLoggedIn
		ret.Msg = "用户不在线"
		return
	}

	if !client.IsLogin() {
		log.Println("心跳接口 用户未登录", client.AppId, client.UserId)
		ret.Code = common.NotLoggedIn
		return
	}

	client.Heartbeat(currentTime)
	ret.Code = common.OK
	return
}
