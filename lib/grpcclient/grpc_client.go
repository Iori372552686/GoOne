/**
* Created by GoLand.
* User: link1st
* Date: 2019-08-03
* Time: 16:43
 */

package grpcclient

import (
	"context"
	"errors"
	"fmt"
	"time"

	`bian/src/bian_newFrame/common`
	`bian/src/bian_newFrame/lib/web/models`
	protobuf `bian/src/bian_newFrame/protobuf/protocol`

	"google.golang.org/grpc"
)

// rpc client
// 给全体用户发送消息
// link::https://github.com/grpc/grpc-go/blob/master/examples/helloworld/greeter_client/main.go
func SendMsgAll(server *models.Server, seq string, appId uint32, userId string, cmd string, message string) (sendMsgId string, err error) {
	// Set up a connection to the server.
	conn, err := grpc.Dial(server.String(), grpc.WithInsecure())
	if err != nil {
		fmt.Println("连接失败", server.String())

		return
	}
	defer conn.Close()

	c := protobuf.NewAccServerClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	req := protobuf.SendMsgAllReq{
		Seq:    seq,
		AppId:  appId,
		UserId: userId,
		Cms:    cmd,
		Msg:    message,
	}
	rsp, err := c.SendMsgAll(ctx, &req)
	if err != nil {
		fmt.Println("给全体用户发送消息", err)

		return
	}

	if rsp.GetRetCode() != common.OK {
		fmt.Println("给全体用户发送消息", rsp.String())
		err = errors.New(fmt.Sprintf("发送消息失败 code:%d", rsp.GetRetCode()))

		return
	}

	sendMsgId = rsp.GetSendMsgId()
	fmt.Println("给全体用户发送消息 成功:", sendMsgId)

	return
}


// rpc client
// 发送消息
// link::https://github.com/grpc/grpc-go/blob/master/examples/helloworld/greeter_client/main.go
func SendMsg(server *models.Server, seq string, appId uint32, userId string, cmd string, msgType string, message string) (sendMsgId string, err error) {
	// Set up a connection to the server.
	conn, err := grpc.Dial(server.String(), grpc.WithInsecure())
	if err != nil {
		fmt.Println("连接失败", server.String())

		return
	}
	defer conn.Close()

	c := protobuf.NewAccServerClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	req := protobuf.SendMsgReq{
		Seq:     seq,
		AppId:   appId,
		UserId:  userId,
		Cms:     cmd,
		Type:    msgType,
		Msg:     message,
		IsLocal: false,
	}
	rsp, err := c.SendMsg(ctx, &req)
	if err != nil {
		fmt.Println("发送消息", err)

		return
	}

	if rsp.GetRetCode() != common.OK {
		fmt.Println("发送消息", rsp.String())
		err = errors.New(fmt.Sprintf("发送消息失败 code:%d", rsp.GetRetCode()))

		return
	}

	sendMsgId = rsp.GetSendMsgId()
	fmt.Println("发送消息 成功:", sendMsgId)

	return
}
