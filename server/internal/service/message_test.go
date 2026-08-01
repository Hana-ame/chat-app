// Package service_test 覆盖业务逻辑层:全部 service 方法、权限(成员/所有者/
// 管理员)、错误映射、context 取消传播、DB 错误注入(WithTx 回滚)、
// StreamService 全生命周期、并发场景。
//
// 运行方式: cd server && go test ./internal/service/
// 说明:AI 上游用 httptest 假 SSE server(见 stream_test.go),DB 为真实
// SQLite 临时库。
package service_test

import (
	"context"
	"testing"

	"github.com/Hana-ame/chat-app/server/internal/models"
	"github.com/Hana-ame/chat-app/server/internal/service"
	"github.com/Hana-ame/chat-app/server/internal/testutil"
)

func TestMessageService_List(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_lst@x.com", "MsgLst")
	chat := createTestChat(t, f, "MsgList", a, []string{a})
	f.DB.CreateMessage(f.Ctx(), chat.ID, a, "Hello", nil, nil)
	f.DB.CreateMessage(f.Ctx(), chat.ID, a, "World", nil, nil)

	msgs, err := f.Server.Services.Message.List(f.Ctx(), chat.ID, a, "", 10)
	testutil.RequireNoError(t, err)
	testutil.RequireEqual(t, len(msgs), 2)
}

func TestMessageService_List_NotMember(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_lst2@x.com", "MsgLst2")
	b := createTestUser(t, f, "msg_lst3@x.com", "MsgLst3")
	chat := createTestChat(t, f, "MsgList2", a, []string{a})

	_, err := f.Server.Services.Message.List(f.Ctx(), chat.ID, b, "", 10)
	testutil.RequireEqual(t, err, service.ErrForbidden)
}

func TestMessageService_Send(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_snd@x.com", "MsgSnd")
	chat := createTestChat(t, f, "MsgSend", a, []string{a})

	msg, err := f.Server.Services.Message.Send(f.Ctx(), chat.ID, a, "Hello, world!", nil)
	testutil.RequireNoError(t, err)
	testutil.RequireEqual(t, msg.Content, "Hello, world!")
	testutil.RequireEqual(t, msg.UserID, a)
	testutil.RequireEqual(t, msg.ChatID, chat.ID)
}

func TestMessageService_Send_NotMember(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_snd2@x.com", "MsgSnd2")
	b := createTestUser(t, f, "msg_snd3@x.com", "MsgSnd3")
	chat := createTestChat(t, f, "MsgSend2", a, []string{a})

	_, err := f.Server.Services.Message.Send(f.Ctx(), chat.ID, b, "test", nil)
	testutil.RequireEqual(t, err, service.ErrForbidden)
}

func TestMessageService_Send_EmptyContent(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_snd4@x.com", "MsgSnd4")
	chat := createTestChat(t, f, "MsgSend3", a, []string{a})

	_, err := f.Server.Services.Message.Send(f.Ctx(), chat.ID, a, "", nil)
	testutil.RequireEqual(t, err, service.ErrInvalidInput)
}

func TestMessageService_Send_WhitespaceContent(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_snd5@x.com", "MsgSnd5")
	chat := createTestChat(t, f, "MsgSend4", a, []string{a})

	_, err := f.Server.Services.Message.Send(f.Ctx(), chat.ID, a, "  ", nil)
	testutil.RequireEqual(t, err, service.ErrInvalidInput)
}

func TestMessageService_Send_AttachmentOnly(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_att@x.com", "MsgAtt")
	chat := createTestChat(t, f, "MsgAttTest", a, []string{a})

	atts := []models.Attachment{
		{Filename: "file.pdf", MimeType: "application/pdf", Size: 100, URL: "http://localhost:8080/api/local/1234567890/file.pdf"},
	}
	msg, err := f.Server.Services.Message.Send(f.Ctx(), chat.ID, a, "", atts)
	testutil.RequireNoError(t, err)
	testutil.RequireEqual(t, msg.AttachmentCount, 1)
}

func TestMessageService_Send_InvalidAttachmentURL(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_att2@x.com", "MsgAtt2")
	chat := createTestChat(t, f, "MsgAttTest2", a, []string{a})

	atts := []models.Attachment{
		{Filename: "file.pdf", URL: "https://evil.com/file.pdf"},
	}
	_, err := f.Server.Services.Message.Send(f.Ctx(), chat.ID, a, "test", atts)
	testutil.RequireEqual(t, err, service.ErrInvalidInput)
}

func TestMessageService_Send_AttachmentNoURL(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_att3@x.com", "MsgAtt3")
	chat := createTestChat(t, f, "MsgAttTest3", a, []string{a})

	atts := []models.Attachment{
		{Filename: "", URL: ""},
	}
	_, err := f.Server.Services.Message.Send(f.Ctx(), chat.ID, a, "test", atts)
	testutil.RequireEqual(t, err, service.ErrInvalidInput)
}

func TestMessageService_Send_DefaultMimeType(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_att4@x.com", "MsgAtt4")
	chat := createTestChat(t, f, "MsgAttTest4", a, []string{a})

	atts := []models.Attachment{
		{Filename: "file.bin", URL: "http://localhost:8080/api/local/1234567890/file.bin"},
	}
	msg, err := f.Server.Services.Message.Send(f.Ctx(), chat.ID, a, "test", atts)
	testutil.RequireNoError(t, err)
	testutil.RequireEqual(t, msg.AttachmentCount, 1)
}

func TestMessageService_Send_Mentions(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_men@x.com", "MsgMen")
	b := createTestUser(t, f, "msg_men2@x.com", "MsgMen2")
	chat := createTestChat(t, f, "MsgMentions", a, []string{a, b})

	content := "Hey <@" + b + "> check this out!"
	msg, err := f.Server.Services.Message.Send(f.Ctx(), chat.ID, a, content, nil)
	testutil.RequireNoError(t, err)
	testutil.RequireEqual(t, msg.MentionCount, 1)
}

func TestMessageService_Send_ContentTooLong(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_long@x.com", "MsgLong")
	chat := createTestChat(t, f, "MsgLongTest", a, []string{a})

	buf := make([]byte, 4001)
	for i := range buf {
		buf[i] = 'a'
	}
	_, err := f.Server.Services.Message.Send(f.Ctx(), chat.ID, a, string(buf), nil)
	testutil.RequireEqual(t, err, service.ErrContentTooLong)
}

func TestMessageService_Edit_Success(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_ed@x.com", "MsgEd")
	chat := createTestChat(t, f, "MsgEdit", a, []string{a})

	msg, _ := f.DB.CreateMessage(f.Ctx(), chat.ID, a, "original", nil, nil)
	edited, err := f.Server.Services.Message.Edit(f.Ctx(), chat.ID, msg.ID, a, "edited")
	testutil.RequireNoError(t, err)
	testutil.RequireEqual(t, edited.Content, "edited")
	testutil.RequireNotNil(t, edited.EditedAt)
}

func TestMessageService_Edit_NotMember(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_ed2@x.com", "MsgEd2")
	b := createTestUser(t, f, "msg_ed3@x.com", "MsgEd3")
	chat := createTestChat(t, f, "MsgEdit2", a, []string{a})
	msg, _ := f.DB.CreateMessage(f.Ctx(), chat.ID, a, "test", nil, nil)

	_, err := f.Server.Services.Message.Edit(f.Ctx(), chat.ID, msg.ID, b, "new")
	testutil.RequireEqual(t, err, service.ErrForbidden)
}

func TestMessageService_Edit_WrongChat(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_ed4@x.com", "MsgEd4")
	chat1 := createTestChat(t, f, "MsgEdit3", a, []string{a})
	chat2 := createTestChat(t, f, "MsgEdit4", a, []string{a})

	msg, _ := f.DB.CreateMessage(f.Ctx(), chat1.ID, a, "test", nil, nil)
	_, err := f.Server.Services.Message.Edit(f.Ctx(), chat2.ID, msg.ID, a, "new")
	testutil.RequireEqual(t, err, service.ErrInvalidInput)
}

func TestMessageService_Delete_OwnMessage(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_del@x.com", "MsgDel")
	chat := createTestChat(t, f, "MsgDelete", a, []string{a})
	msg, _ := f.DB.CreateMessage(f.Ctx(), chat.ID, a, "delete me", nil, nil)

	err := f.Server.Services.Message.Delete(f.Ctx(), chat.ID, msg.ID, a)
	testutil.RequireNoError(t, err)
	// Soft delete: message still exists but has DeletedAt set and content cleared
	m, err := f.DB.GetMessage(f.Ctx(), msg.ID)
	testutil.RequireNoError(t, err)
	testutil.RequireNotNil(t, m.DeletedAt)
	testutil.RequireEqual(t, m.Content, "")
}

func TestMessageService_Delete_OtherUserMessage_Forbidden(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_del2@x.com", "MsgDel2")
	b := createTestUser(t, f, "msg_del3@x.com", "MsgDel3")
	chat := createTestChat(t, f, "MsgDelete2", a, []string{a, b})
	msg, _ := f.DB.CreateMessage(f.Ctx(), chat.ID, a, "test", nil, nil)

	err := f.Server.Services.Message.Delete(f.Ctx(), chat.ID, msg.ID, b)
	testutil.RequireEqual(t, err, service.ErrForbidden)
}

func TestMessageService_Delete_WrongChat(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_del4@x.com", "MsgDel4")
	chat1 := createTestChat(t, f, "MsgDelete3", a, []string{a})
	chat2 := createTestChat(t, f, "MsgDelete4", a, []string{a})
	msg, _ := f.DB.CreateMessage(f.Ctx(), chat1.ID, a, "test", nil, nil)

	err := f.Server.Services.Message.Delete(f.Ctx(), chat2.ID, msg.ID, a)
	testutil.RequireEqual(t, err, service.ErrInvalidInput)
}

func TestMessageService_MarkRead(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_mr@x.com", "MsgMR")
	chat := createTestChat(t, f, "MsgMRTest", a, []string{a})
	err := f.Server.Services.Message.MarkRead(f.Ctx(), chat.ID, a)
	testutil.RequireNoError(t, err)
}

func TestMessageService_MarkRead_NotMember(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_mr2@x.com", "MsgMR2")
	b := createTestUser(t, f, "msg_mr3@x.com", "MsgMR3")
	chat := createTestChat(t, f, "MsgMRTest2", a, []string{a})

	err := f.Server.Services.Message.MarkRead(f.Ctx(), chat.ID, b)
	testutil.RequireEqual(t, err, service.ErrForbidden)
}

func TestMessageService_Send_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "sndc@x.com", "SndC")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := f.Server.Services.Message.Send(ctx, "chatid", a, "test", nil)
	testutil.RequireError(t, err)
}

func TestMessageService_Edit_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "edtc@x.com", "EdtC")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := f.Server.Services.Message.Edit(ctx, "chatid", "msgid", a, "new")
	testutil.RequireError(t, err)
}

func TestMessageService_Delete_NotMember(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_del8@x.com", "MsgDel8")
	b := createTestUser(t, f, "msg_del9@x.com", "MsgDel9")
	chat := createTestChat(t, f, "MsgDelete9", a, []string{a})
	msg, _ := f.DB.CreateMessage(context.Background(), chat.ID, a, "test", nil, nil)

	err := f.Server.Services.Message.Delete(context.Background(), chat.ID, msg.ID, b)
	testutil.RequireEqual(t, err, service.ErrForbidden)
}

func TestMessageService_Delete_BroadcastError(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_del10@x.com", "MsgDel10")
	chat := createTestChat(t, f, "MsgDelete10", a, []string{a})
	msg, _ := f.DB.CreateMessage(context.Background(), chat.ID, a, "test", nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := f.Server.Services.Message.Delete(ctx, chat.ID, msg.ID, a)
	testutil.RequireError(t, err)
}

func TestMessageService_MarkRead_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "mrkc@x.com", "MrkC")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := f.Server.Services.Message.MarkRead(ctx, "chatid", a)
	testutil.RequireError(t, err)
}

func TestMessageService_List_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msglc@x.com", "MsgLC")
	chat := createTestChat(t, f, "MsgLCTest", a, []string{a})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := f.Server.Services.Message.List(ctx, chat.ID, a, "", 10)
	testutil.RequireError(t, err)
}

func TestMessageService_Edit_Nonexistent(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_ed5@x.com", "MsgEd5")
	chat := createTestChat(t, f, "MsgEdit5", a, []string{a})
	_, err := f.Server.Services.Message.Edit(context.Background(), chat.ID, "nonexistent", a, "edited")
	testutil.RequireEqual(t, err, service.ErrNotFound)
}

func TestMessageService_Edit_EmptyContent(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_ed6@x.com", "MsgEd6")
	chat := createTestChat(t, f, "MsgEdit6", a, []string{a})
	msg, _ := f.DB.CreateMessage(context.Background(), chat.ID, a, "original", nil, nil)
	_, err := f.Server.Services.Message.Edit(context.Background(), chat.ID, msg.ID, a, "")
	testutil.RequireError(t, err)
}

func TestMessageService_Delete_NonexistentMessage(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_del11@x.com", "MsgDel11")
	chat := createTestChat(t, f, "MsgDelete11", a, []string{a})
	err := f.Server.Services.Message.Delete(context.Background(), chat.ID, "nonexistent", a)
	testutil.RequireError(t, err)
}

func TestMessageService_Delete_OtherUserMessageAsOwner(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_del12@x.com", "MsgDel12")
	b := createTestUser(t, f, "msg_del13@x.com", "MsgDel13")
	chat := createTestChat(t, f, "MsgDelete12", a, []string{a, b})
	msg, _ := f.DB.CreateMessage(context.Background(), chat.ID, b, "test", nil, nil)
	err := f.Server.Services.Message.Delete(context.Background(), chat.ID, msg.ID, a)
	testutil.RequireNoError(t, err)
}

func TestMessageService_Send_CreateMessageError(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_cme@x.com", "MsgCME")
	chat := createTestChat(t, f, "MsgCMETest", a, []string{a})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := f.Server.Services.Message.Send(ctx, chat.ID, a, "test", nil)
	testutil.RequireError(t, err)
}
