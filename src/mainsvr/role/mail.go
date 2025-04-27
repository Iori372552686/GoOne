package role

import (
	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/module/misc"
	g1_protocol "github.com/Iori372552686/game_protocol"
)

// 添加邮件，这里一般是在trans中执行的，所以要加上cmd_handle参数
func (r *Role) MailAdd(c cmd_handler.IContext, mailType int32, confID int32, attach *[]*g1_protocol.PbItem) int {
	mail := &g1_protocol.PbMail{}
	mail.Type = mailType
	mail.ConfId = confID
	mail.CreateTime = r.Now()
	if attach != nil {
		mail.AttachList = *attach
	}
	mail.Sender = c.Uid()

	req := &g1_protocol.MailInnerAddMailReq{}
	req.MailList = append(req.MailList, mail)
	rsp := &g1_protocol.MailInnerAddMailRsp{Ret: &g1_protocol.Ret{}}
	err := c.CallMsgBySvrType(misc.ServerType_MailSvr, g1_protocol.CMD_MAIL_INNER_ADD_MAIL_REQ, req, rsp)
	if err != nil {
		c.Errorf("send mail error, %v", err)
		return -1
	}

	return int(rsp.Ret.Code)
}
