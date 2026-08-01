// Package testutil_test 覆盖黑盒 HTTP 集成测试:通过 testutil.Fixture 装配
// 完整服务栈(真实 SQLite + httptest server),验证建群/发消息/DM/成员管理/
// 反应/置顶/公开聊天/SSE/上传附件及全部错误路径。
//
// 运行方式: cd server && go test ./internal/testutil/
package testutil_test

import (
	"bufio"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/testutil"
)

func TestCreateGroupChatAndSendMessage(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "alice@ch.t", "Alice", "password123")
	bob := f.Register(t, "bob@ch.t", "Bob", "password123")

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "Dev Team", "member_ids": []string{bob.UserID},
	})
	defer res.Body.Close()
	testutil.RequireStatus(t, res, 201)
	var chat struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	}
	testutil.RequireNoError(t, json.NewDecoder(res.Body).Decode(&chat))
	testutil.RequireTrue(t, chat.Name == "Dev Team" && chat.Type == "group", "chat wrong")

	sendRes := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", alice.AccessToken, map[string]string{
		"content": "hello team!",
	})
	defer sendRes.Body.Close()
	testutil.RequireStatus(t, sendRes, 201)
	var msg struct {
		ID      string `json:"id"`
		Content string `json:"content"`
		ChatID  string `json:"chat_id"`
	}
	testutil.RequireNoError(t, json.NewDecoder(sendRes.Body).Decode(&msg))
	testutil.RequireEqual(t, msg.Content, "hello team!")

	listRes := f.Do(t, "GET", "/api/chats/"+chat.ID+"/messages?limit=50", alice.AccessToken, nil)
	defer listRes.Body.Close()
	testutil.RequireStatus(t, listRes, 200)
	var listResp struct {
		Messages []map[string]any `json:"messages"`
	}
	json.NewDecoder(listRes.Body).Decode(&listResp)
	testutil.RequireEqual(t, len(listResp.Messages), 1)

	editRes := f.Do(t, "PATCH", "/api/chats/"+chat.ID+"/messages/"+msg.ID, alice.AccessToken, map[string]string{
		"content": "edited hello!",
	})
	defer editRes.Body.Close()
	testutil.RequireStatus(t, editRes, 200)
}

func TestCreateDM(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "dm1@dm.t", "AliceDM", "password123")
	bob := f.Register(t, "dm2@dm.t", "BobDM", "password123")

	res := f.Do(t, "POST", "/api/dms", alice.AccessToken, map[string]string{
		"user_id": bob.UserID,
	})
	defer res.Body.Close()
	testutil.RequireStatusAny(t, res, 201, 200)

	res2 := f.Do(t, "POST", "/api/dms", alice.AccessToken, map[string]string{
		"user_id": bob.UserID,
	})
	defer res2.Body.Close()
	testutil.RequireStatus(t, res2, 200)

	res3 := f.Do(t, "POST", "/api/dms", alice.AccessToken, map[string]string{
		"user_id": alice.UserID,
	})
	defer res3.Body.Close()
	testutil.RequireTrue(t, res3.StatusCode != 201 && res3.StatusCode != 200, "self dm should fail")
}

func TestAddRemoveMembers(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "owner@mem.t", "Owner", "password123")
	bob := f.Register(t, "joiner@mem.t", "Joiner", "password123")
	carol := f.Register(t, "guest@mem.t", "Guest", "password123")

	var chatID string
	{
		res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
			"type": "group", "name": "Member Test", "member_ids": []string{bob.UserID},
		})
		defer res.Body.Close()
		var c map[string]any
		json.NewDecoder(res.Body).Decode(&c)
		chatID = c["id"].(string)
	}

	addRes := f.Do(t, "POST", "/api/chats/"+chatID+"/members", alice.AccessToken, map[string]string{
		"user_id": carol.UserID,
	})
	defer addRes.Body.Close()
	testutil.RequireStatus(t, addRes, 200)

	membersRes := f.Do(t, "GET", "/api/chats/"+chatID+"/members", alice.AccessToken, nil)
	defer membersRes.Body.Close()
	var membersResp struct {
		Members []map[string]any `json:"members"`
	}
	json.NewDecoder(membersRes.Body).Decode(&membersResp)
	testutil.RequireEqual(t, len(membersResp.Members), 3)

	removeRes := f.Do(t, "DELETE", "/api/chats/"+chatID+"/members/"+carol.UserID, alice.AccessToken, nil)
	defer removeRes.Body.Close()
	testutil.RequireStatus(t, removeRes, 200)

	membersRes = f.Do(t, "GET", "/api/chats/"+chatID+"/members", alice.AccessToken, nil)
	defer membersRes.Body.Close()
	json.NewDecoder(membersRes.Body).Decode(&membersResp)
	testutil.RequireEqual(t, len(membersResp.Members), 2)
}

func TestReactionsFlow(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "r1@rx.t", "AliceRx", "password123")
	bob := f.Register(t, "r2@rx.t", "BobRx", "password123")

	var chatID, msgID string
	{
		res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
			"type": "group", "name": "Rxn", "member_ids": []string{bob.UserID},
		})
		var c map[string]any
		json.NewDecoder(res.Body).Decode(&c)
		res.Body.Close()
		chatID = c["id"].(string)

		res2 := f.Do(t, "POST", "/api/chats/"+chatID+"/messages", alice.AccessToken, map[string]string{
			"content": "react to me",
		})
		var m map[string]any
		json.NewDecoder(res2.Body).Decode(&m)
		res2.Body.Close()
		msgID = m["id"].(string)
	}

	addRes := f.Do(t, "PUT", "/api/chats/"+chatID+"/messages/"+msgID+"/reactions/%F0%9F%91%8D", alice.AccessToken, nil)
	addRes.Body.Close()
	testutil.RequireStatus(t, addRes, 200)

	addRes2 := f.Do(t, "PUT", "/api/chats/"+chatID+"/messages/"+msgID+"/reactions/%F0%9F%91%8D", bob.AccessToken, nil)
	addRes2.Body.Close()
	testutil.RequireStatus(t, addRes2, 200)

	delRes := f.Do(t, "DELETE", "/api/chats/"+chatID+"/messages/"+msgID+"/reactions/%F0%9F%91%8D", alice.AccessToken, nil)
	delRes.Body.Close()
	testutil.RequireStatus(t, delRes, 200)

	listRes := f.Do(t, "GET", "/api/chats/"+chatID+"/messages?limit=5", alice.AccessToken, nil)
	defer listRes.Body.Close()
	var listResp struct {
		Messages []struct {
			Reactions []struct {
				Emoji string `json:"emoji"`
				Count int    `json:"count"`
			} `json:"reactions"`
		} `json:"messages"`
	}
	json.NewDecoder(listRes.Body).Decode(&listResp)
	testutil.RequireTrue(t, len(listResp.Messages) == 1 && len(listResp.Messages[0].Reactions) == 1, "want 1 reaction group, got %d", len(listResp.Messages[0].Reactions))
	testutil.RequireEqual(t, listResp.Messages[0].Reactions[0].Count, 1)
}

func TestUpdateProfile(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "prof@p.t", "ProfUser", "password123")

	res := f.Do(t, "PATCH", "/api/users/me", alice.AccessToken, map[string]string{
		"username": "NewProfUser",
	})
	defer res.Body.Close()
	testutil.RequireStatus(t, res, 200)
	var u map[string]any
	json.NewDecoder(res.Body).Decode(&u)
	testutil.RequireEqual(t, u["username"], "NewProfUser")
}

func TestSearchUsers(t *testing.T) {
	f := testutil.New(t)
	f.Register(t, "search1@s.t", "SearchTarget", "password123")
	alice := f.Register(t, "search2@s.t", "AliceSearch", "password123")

	res := f.Do(t, "GET", "/api/users?q=archt", alice.AccessToken, nil)
	defer res.Body.Close()

	var resp struct {
		Users []map[string]any `json:"users"`
	}
	json.NewDecoder(res.Body).Decode(&resp)
	found := false
	for _, u := range resp.Users {
		if u["username"] == "SearchTarget" {
			found = true
		}
	}
	testutil.RequireTrue(t, found, "search didn't find target user")
	for _, u := range resp.Users {
		testutil.RequireNotEqual(t, u["id"], alice.UserID)
	}
}

func TestDeleteMessageAsAdmin(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "admin@moonchan.xyz", "Nanaka", "password123")
	bob := f.Register(t, "user@d.t", "User", "password123")

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "Admin Delete", "member_ids": []string{bob.UserID},
	})
	var c map[string]any
	json.NewDecoder(res.Body).Decode(&c)
	res.Body.Close()
	chatID := c["id"].(string)

	res2 := f.Do(t, "POST", "/api/chats/"+chatID+"/messages", bob.AccessToken, map[string]string{
		"content": "bob's message",
	})
	var m map[string]any
	json.NewDecoder(res2.Body).Decode(&m)
	res2.Body.Close()
	msgID := m["id"].(string)

	delRes := f.Do(t, "DELETE", "/api/chats/"+chatID+"/messages/"+msgID, alice.AccessToken, nil)
	defer delRes.Body.Close()
	testutil.RequireStatus(t, delRes, 200)
}

func TestLeaveGroupChat(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "leave1@lv.t", "LeaveMe", "password123")
	bob := f.Register(t, "leave2@lv.t", "StayHere", "password123")

	res := f.Do(t, "POST", "/api/chats", bob.AccessToken, map[string]any{
		"type": "group", "name": "Leave Test", "member_ids": []string{alice.UserID},
	})
	var c map[string]any
	json.NewDecoder(res.Body).Decode(&c)
	res.Body.Close()
	chatID := c["id"].(string)

	leaveRes := f.Do(t, "DELETE", "/api/chats/"+chatID+"/members/"+alice.UserID, alice.AccessToken, nil)
	defer leaveRes.Body.Close()
	testutil.RequireStatus(t, leaveRes, 200)

	accessRes := f.Do(t, "GET", "/api/chats/"+chatID+"/messages?limit=1", alice.AccessToken, nil)
	defer accessRes.Body.Close()
	testutil.RequireStatus(t, accessRes, 403)
}

func TestAuthEndpoints(t *testing.T) {
	f := testutil.New(t)

	t.Run("register accepts any input (validations removed)", func(t *testing.T) {
		res := f.Do(t, "POST", "/api/auth/register", "", map[string]string{
			"email": "not-an-email", "username": "a", "password": "password123",
		})
		res.Body.Close()
		testutil.RequireStatus(t, res, 200)
	})

	t.Run("login with wrong password", func(t *testing.T) {
		f.Register(t, "wrongpw@t.com", "WrongPW", "testtest123")
		res := f.Do(t, "POST", "/api/auth/login", "", map[string]string{
			"email": "wrongpw@t.com", "password": "badbadbad",
		})
		res.Body.Close()
		testutil.RequireStatus(t, res, 401)
	})

	t.Run("refresh with garbage token", func(t *testing.T) {
		res := f.DoWithCookie(t, "POST", "/api/auth/refresh", "", "refresh_token", "not-a-real-token", nil)
		res.Body.Close()
		testutil.RequireStatus(t, res, 401)
	})

	t.Run("logout cleans up refresh token", func(t *testing.T) {
		s := f.Register(t, "logout@t.com", "LogoutUser", "testtest123")
		res := f.Do(t, "POST", "/api/auth/logout", s.AccessToken, nil)
		res.Body.Close()
		testutil.RequireStatus(t, res, 200)
		refreshRes := f.DoWithCookie(t, "POST", "/api/auth/refresh", "", "refresh_token", s.RefreshToken, nil)
		refreshRes.Body.Close()
		testutil.RequireStatus(t, refreshRes, 401)
	})

	t.Run("get me returns user", func(t *testing.T) {
		s := f.Register(t, "me@t.com", "MeUser", "testtest123")
		res := f.Do(t, "GET", "/api/users/me", s.AccessToken, nil)
		defer res.Body.Close()
		var u map[string]any
		json.NewDecoder(res.Body).Decode(&u)
		testutil.RequireEqual(t, u["username"], "MeUser")
	})
}

func TestListChatsWithUnreads(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "listchat1@lc.t", "AliceLC", "testtest123")
	bob := f.Register(t, "listchat2@lc.t", "BobLC", "testtest123")

	{
		res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
			"type": "group", "name": "Chat A", "member_ids": []string{bob.UserID},
		})
		var c map[string]any
		json.NewDecoder(res.Body).Decode(&c)
		res.Body.Close()
		chatID := c["id"].(string)

		res2 := f.Do(t, "POST", "/api/chats/"+chatID+"/messages", bob.AccessToken, map[string]string{
			"content": "hello from bob",
		})
		res2.Body.Close()
	}

	listRes := f.Do(t, "GET", "/api/chats/my", alice.AccessToken, nil)
	defer listRes.Body.Close()
	testutil.RequireStatus(t, listRes, 200)
	var listResp struct {
		Chats []struct {
			Name        string `json:"name"`
			UnreadCount int    `json:"unread_count"`
			LastMessage *struct {
				Content string `json:"content"`
			} `json:"last_message"`
		} `json:"chats"`
	}
	json.NewDecoder(listRes.Body).Decode(&listResp)
	testutil.RequireTrue(t, len(listResp.Chats) >= 1, "no chats returned")
	for _, c := range listResp.Chats {
		if c.Name == "Chat A" {
			if c.LastMessage != nil && c.LastMessage.Content == "hello from bob" {
				if c.UnreadCount > 0 {
					t.Log("unread count works")
				}
				return
			}
		}
	}
}

func TestRenameChatOnlyOwner(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "owner@rn.t", "OwnerR", "testtest123")
	bob := f.Register(t, "user@rn.t", "UserR", "testtest123")

	var chatID string
	{
		res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
			"type": "group", "name": "RenameMe", "member_ids": []string{bob.UserID},
		})
		var c map[string]any
		json.NewDecoder(res.Body).Decode(&c)
		res.Body.Close()
		chatID = c["id"].(string)
	}

	renameRes := f.Do(t, "PATCH", "/api/chats/"+chatID, bob.AccessToken, map[string]string{
		"name": "Hacked",
	})
	renameRes.Body.Close()
	testutil.RequireStatus(t, renameRes, 403)

	renameRes2 := f.Do(t, "PATCH", "/api/chats/"+chatID, alice.AccessToken, map[string]string{
		"name": "Renamed",
	})
	renameRes2.Body.Close()
	testutil.RequireStatus(t, renameRes2, 200)
}

func TestChatForbidden(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "a@fc.t", "Alice", "testtest123")
	bob := f.Register(t, "b@fc.t", "Bob", "testtest123")

	var chatID string
	{
		res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
			"type": "group", "name": "Private", "member_ids": []string{bob.UserID},
		})
		var c map[string]any
		json.NewDecoder(res.Body).Decode(&c)
		res.Body.Close()
		chatID = c["id"].(string)
	}
	carol := f.Register(t, "c@fc.t", "Carol", "testtest123")
	sendRes := f.Do(t, "POST", "/api/chats/"+chatID+"/messages", carol.AccessToken, map[string]string{
		"content": "interloper",
	})
	sendRes.Body.Close()
	testutil.RequireStatus(t, sendRes, 403)
}

func TestMarkRead(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "read1@rd.t", "Reader", "testtest123")

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "ReadTest", "member_ids": []string{},
	})
	var c map[string]any
	json.NewDecoder(res.Body).Decode(&c)
	res.Body.Close()
	chatID := c["id"].(string)

	msgRes := f.Do(t, "POST", "/api/chats/"+chatID+"/messages", alice.AccessToken, map[string]string{
		"content": "msg1",
	})
	var m map[string]any
	json.NewDecoder(msgRes.Body).Decode(&m)
	msgRes.Body.Close()

	readRes := f.Do(t, "POST", "/api/chats/"+chatID+"/read", alice.AccessToken, map[string]string{
		"message_id": m["id"].(string),
	})
	readRes.Body.Close()
	testutil.RequireStatus(t, readRes, 200)
}

func TestDeleteChatOnlyOwner(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "del1@dt.t", "DelOwner", "testtest123")
	bob := f.Register(t, "del2@dt.t", "DelUser", "testtest123")

	var chatID string
	{
		res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
			"type": "group", "name": "DeleteTest", "member_ids": []string{bob.UserID},
		})
		var c map[string]any
		json.NewDecoder(res.Body).Decode(&c)
		res.Body.Close()
		chatID = c["id"].(string)
	}

	delRes := f.Do(t, "DELETE", "/api/chats/"+chatID, bob.AccessToken, nil)
	delRes.Body.Close()
	testutil.RequireStatus(t, delRes, 403)
	delRes2 := f.Do(t, "DELETE", "/api/chats/"+chatID, alice.AccessToken, nil)
	delRes2.Body.Close()
	testutil.RequireStatus(t, delRes2, 200)
}

func TestConcurrentRegister(t *testing.T) {
	f := testutil.New(t)
	results := make(chan int, 5)
	for i := 0; i < 5; i++ {
		go func() {
			res := f.Do(t, "POST", "/api/auth/register", "", map[string]string{
				"email":    "concurrent@t.com",
				"username": "ConcurrentUser",
				"password": "testtest123",
			})
			res.Body.Close()
			results <- res.StatusCode
		}()
	}
	ok := 0
	conflict := 0
	other := 0
	for i := 0; i < 5; i++ {
		select {
		case code := <-results:
			switch code {
			case 200:
				ok++
			case 409:
				conflict++
			default:
				other++
			}
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for concurrent register")
		}
	}
	testutil.RequireEqual(t, ok, 1)
}

func TestHealthz(t *testing.T) {
	f := testutil.New(t)
	// Send a header so the echo can be verified
	res := f.Do(t, "GET", "/healthz", "", nil)
	defer res.Body.Close()
	testutil.RequireStatus(t, res, http.StatusOK)
	var body map[string]any
	testutil.RequireNoError(t, json.NewDecoder(res.Body).Decode(&body))
	testutil.RequireEqual(t, body["status"], "ok")
	echo, ok := body["echo"].(map[string]any)
	testutil.RequireTrue(t, ok, "expected echo object, got %T", body["echo"])
	testutil.RequireTrue(t, len(echo) > 0, "expected non-empty echo object")
}

func TestUpdateMeUsernameConflict(t *testing.T) {
	f := testutil.New(t)
	_ = f.Register(t, "upa@test.dev", "UserA", "testPass1!")
	b := f.Register(t, "upb@test.dev", "UserB", "testPass1!")

	res := f.Do(t, "PATCH", "/api/users/me", b.AccessToken, map[string]string{
		"username": "UserA",
	})
	defer res.Body.Close()
	testutil.RequireStatus(t, res, 409)

	res2 := f.Do(t, "PATCH", "/api/users/me", b.AccessToken, map[string]string{
		"username": "UserB-renamed",
	})
	defer res2.Body.Close()
	testutil.RequireStatus(t, res2, 200)
	var u struct {
		Username string `json:"username"`
	}
	json.NewDecoder(res2.Body).Decode(&u)
	testutil.RequireEqual(t, u.Username, "UserB-renamed")
}

func TestCreateChatInvalidInput(t *testing.T) {
	f := testutil.New(t)
	s := f.Register(t, "chatbad@test.dev", "ChatBad", "testPass1!")

	res := f.Do(t, "POST", "/api/chats", s.AccessToken, map[string]any{
		"type": "invalid-type", "name": "Test", "member_ids": []string{},
	})
	defer res.Body.Close()
	testutil.RequireStatus(t, res, 400)
}

func TestSendMessageNonMember(t *testing.T) {
	f := testutil.New(t)
	a := f.Register(t, "nmsga@test.dev", "NonMemberA", "testPass1!")
	b := f.Register(t, "nmsgb@test.dev", "NonMemberB", "testPass1!")

	res := f.Do(t, "POST", "/api/chats", a.AccessToken, map[string]any{
		"type": "group", "name": "Exclusive", "member_ids": []string{},
	})
	var chat struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	sendRes := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", b.AccessToken, map[string]string{
		"content": "interloper message",
	})
	defer sendRes.Body.Close()
	testutil.RequireStatus(t, sendRes, 403)
}

func TestSearchUsersEmptyQuery(t *testing.T) {
	f := testutil.New(t)
	s := f.Register(t, "searchqa@test.dev", "SearchQA", "testPass1!")

	res := f.Do(t, "GET", "/api/users", s.AccessToken, nil)
	defer res.Body.Close()
	var resp struct {
		Users []map[string]any `json:"users"`
	}
	json.NewDecoder(res.Body).Decode(&resp)
	testutil.RequireEqual(t, len(resp.Users), 0)
}

func TestSearchUsersExcludesSelf(t *testing.T) {
	f := testutil.New(t)
	s := f.Register(t, "searchqa2@test.dev", "SearchQA", "testPass1!")

	res := f.Do(t, "GET", "/api/users?q=SearchQA", s.AccessToken, nil)
	defer res.Body.Close()
	var resp struct {
		Users []map[string]any `json:"users"`
	}
	json.NewDecoder(res.Body).Decode(&resp)
	for _, u := range resp.Users {
		testutil.RequireNotEqual(t, u["id"], s.UserID)
	}
}

func TestEditMessageNonAuthor(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "edita@e.t", "AliceE", "password123")
	bob := f.Register(t, "editb@e.t", "BobE", "password123")

	var chatID, msgID string
	{
		res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
			"type": "group", "name": "EditTest", "member_ids": []string{bob.UserID},
		})
		var c map[string]any
		json.NewDecoder(res.Body).Decode(&c)
		res.Body.Close()
		chatID = c["id"].(string)
	}
	{
		res := f.Do(t, "POST", "/api/chats/"+chatID+"/messages", alice.AccessToken, map[string]string{
			"content": "alice message",
		})
		var m map[string]any
		json.NewDecoder(res.Body).Decode(&m)
		res.Body.Close()
		msgID = m["id"].(string)
	}

	t.Run("non-author cannot edit", func(t *testing.T) {
		res := f.Do(t, "PATCH", "/api/chats/"+chatID+"/messages/"+msgID, bob.AccessToken, map[string]string{
			"content": "bob edit",
		})
		defer res.Body.Close()
		testutil.RequireStatus(t, res, 404)
	})

	t.Run("message not found", func(t *testing.T) {
		res := f.Do(t, "PATCH", "/api/chats/"+chatID+"/messages/nonexistent-id", alice.AccessToken, map[string]string{
			"content": "edited",
		})
		defer res.Body.Close()
		testutil.RequireStatus(t, res, 404)
	})

	t.Run("chat mismatch", func(t *testing.T) {
		res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
			"type": "group", "name": "OtherChat", "member_ids": []string{},
		})
		var c2 map[string]any
		json.NewDecoder(res.Body).Decode(&c2)
		res.Body.Close()
		otherChatID := c2["id"].(string)
		res2 := f.Do(t, "PATCH", "/api/chats/"+otherChatID+"/messages/"+msgID, alice.AccessToken, map[string]string{
			"content": "wrong chat",
		})
		defer res2.Body.Close()
		testutil.RequireStatus(t, res2, 400)
	})
}

func TestSendMessageWithAttachments(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "att@a.t", "AttAlice", "password123")

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "AttTest", "member_ids": []string{},
	})
	var chat struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	t.Run("attachment missing url", func(t *testing.T) {
		res := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", alice.AccessToken, map[string]any{
			"content": "bad attach",
			"attachments": []map[string]any{
				{"filename": "test.txt"},
			},
		})
		defer res.Body.Close()
		testutil.RequireStatus(t, res, 400)
	})

	t.Run("attachment missing filename", func(t *testing.T) {
		res := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", alice.AccessToken, map[string]any{
			"content": "bad attach",
			"attachments": []map[string]any{
				{"url": "http://localhost:8080/api/local/123/file.png"},
			},
		})
		defer res.Body.Close()
		testutil.RequireStatus(t, res, 400)
	})

	t.Run("attachment invalid url prefix", func(t *testing.T) {
		res := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", alice.AccessToken, map[string]any{
			"content": "bad attach",
			"attachments": []map[string]any{
				{"url": "https://evil.com/virus.exe", "filename": "virus.exe"},
			},
		})
		defer res.Body.Close()
		testutil.RequireStatus(t, res, 400)
	})

	t.Run("attachment mime auto-filled", func(t *testing.T) {
		res := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", alice.AccessToken, map[string]any{
			"content": "with attach",
			"attachments": []map[string]any{
				{"url": "http://localhost:8080/api/local/123/foo.png", "filename": "foo.png"},
			},
		})
		defer res.Body.Close()
		testutil.RequireStatus(t, res, 201)
	})
}

func TestMessageContentTooLong(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "long@l.t", "LongAlice", "password123")

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "LongTest", "member_ids": []string{},
	})
	var chat struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	longContent := string(make([]byte, 4001))
	res2 := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", alice.AccessToken, map[string]string{
		"content": longContent,
	})
	defer res2.Body.Close()
	testutil.RequireStatus(t, res2, 413)
	var errResp struct {
		Error string `json:"error"`
	}
	json.NewDecoder(res2.Body).Decode(&errResp)
	testutil.RequireEqual(t, errResp.Error, "content_too_long")
}

func TestPinMessage(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "pin@a.t", "PinAlice", "password123")
	bob := f.Register(t, "pin@b.t", "PinBob", "password123")
	carol := f.Register(t, "pin@c.t", "PinCarol", "password123")

	var chatID string
	{
		res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
			"type": "group", "name": "PinTest", "member_ids": []string{bob.UserID, carol.UserID},
		})
		var c map[string]any
		json.NewDecoder(res.Body).Decode(&c)
		res.Body.Close()
		chatID = c["id"].(string)
	}

	t.Run("owner can pin", func(t *testing.T) {
		res := f.Do(t, "POST", "/api/chats/"+chatID+"/announcement", alice.AccessToken, map[string]string{
			"content": "pinned message",
		})
		defer res.Body.Close()
		testutil.RequireStatus(t, res, 200)
	})

	t.Run("non-owner cannot pin", func(t *testing.T) {
		res := f.Do(t, "POST", "/api/chats/"+chatID+"/announcement", bob.AccessToken, map[string]string{
			"content": "non-owner pin",
		})
		defer res.Body.Close()
		testutil.RequireStatus(t, res, 403)
	})

	t.Run("small group can pin", func(t *testing.T) {
		res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
			"type": "group", "name": "SmallChat", "member_ids": []string{},
		})
		var small struct {
			ID string `json:"id"`
		}
		json.NewDecoder(res.Body).Decode(&small)
		res.Body.Close()

		res2 := f.Do(t, "POST", "/api/chats/"+small.ID+"/announcement", alice.AccessToken, map[string]string{
			"content": "should succeed",
		})
		defer res2.Body.Close()
		testutil.RequireStatus(t, res2, 200)
	})
}

func TestDeletePinnedChat(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "delpin@a.t", "DelPinA", "password123")
	bob := f.Register(t, "delpin@b.t", "DelPinB", "password123")
	carol := f.Register(t, "delpin@c.t", "DelPinC", "password123")

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "PinDelTest", "member_ids": []string{bob.UserID, carol.UserID},
	})
	var c map[string]any
	json.NewDecoder(res.Body).Decode(&c)
	res.Body.Close()
	chatID := c["id"].(string)

	f.Do(t, "POST", "/api/chats/"+chatID+"/announcement", alice.AccessToken, map[string]string{
		"content": "to delete",
	})

	t.Run("owner can clear pin", func(t *testing.T) {
		res2 := f.Do(t, "DELETE", "/api/chats/"+chatID+"/announcement", alice.AccessToken, nil)
		defer res2.Body.Close()
		testutil.RequireStatus(t, res2, 200)
	})

	f.Do(t, "POST", "/api/chats/"+chatID+"/announcement", alice.AccessToken, map[string]string{
		"content": "pin again",
	})

	t.Run("non-member cannot clear pin", func(t *testing.T) {
		dave := f.Register(t, "delpin@d.t", "DelPinD", "password123")
		res3 := f.Do(t, "DELETE", "/api/chats/"+chatID+"/announcement", dave.AccessToken, nil)
		defer res3.Body.Close()
		testutil.RequireStatus(t, res3, 403)
	})

	t.Run("regular member cannot clear pin", func(t *testing.T) {
		res4 := f.Do(t, "DELETE", "/api/chats/"+chatID+"/announcement", bob.AccessToken, nil)
		defer res4.Body.Close()
		testutil.RequireStatus(t, res4, 403)
	})
}

func TestChatVisibilityAndPublicList(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "vis@a.t", "VisAlice", "password123")

	var publicID, privateID string
	{
		res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
			"type": "group", "name": "PublicChat", "visibility": "public", "member_ids": []string{},
		})
		var c map[string]any
		json.NewDecoder(res.Body).Decode(&c)
		res.Body.Close()
		publicID = c["id"].(string)
	}
	{
		res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
			"type": "group", "name": "UnlistedChat", "visibility": "unlisted", "member_ids": []string{},
		})
		var c map[string]any
		json.NewDecoder(res.Body).Decode(&c)
		res.Body.Close()
		_ = c["id"].(string)
	}
	{
		res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
			"type": "group", "name": "PrivateChat", "visibility": "private", "member_ids": []string{},
		})
		var c map[string]any
		json.NewDecoder(res.Body).Decode(&c)
		res.Body.Close()
		privateID = c["id"].(string)
	}

	publicRes := f.Do(t, "GET", "/api/chats/public", alice.AccessToken, nil)
	defer publicRes.Body.Close()
	testutil.RequireStatus(t, publicRes, 200)
	var listResp struct {
		Chats []struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			Visibility string `json:"visibility"`
		} `json:"chats"`
	}
	json.NewDecoder(publicRes.Body).Decode(&listResp)
	foundPublic := false
	foundPrivate := false
	for _, ch := range listResp.Chats {
		if ch.ID == publicID {
			foundPublic = true
		}
		if ch.ID == privateID {
			foundPrivate = true
		}
	}
	testutil.RequireTrue(t, foundPublic, "public chat not in public list")
	testutil.RequireFalse(t, foundPrivate, "private chat should not be in public list")
}

func TestJoinPublicChat(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "join@a.t", "JoinAlice", "password123")
	bob := f.Register(t, "join@b.t", "JoinBob", "password123")

	var publicChatID string
	{
		res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
			"type": "group", "name": "Joinable", "visibility": "public", "member_ids": []string{},
		})
		var c map[string]any
		json.NewDecoder(res.Body).Decode(&c)
		res.Body.Close()
		publicChatID = c["id"].(string)
	}

	t.Run("join public chat", func(t *testing.T) {
		res := f.Do(t, "POST", "/api/chats/"+publicChatID+"/join", bob.AccessToken, nil)
		defer res.Body.Close()
		testutil.RequireStatus(t, res, 200)
	})

	t.Run("appears in member list after join", func(t *testing.T) {
		memRes := f.Do(t, "GET", "/api/chats/"+publicChatID+"/members", bob.AccessToken, nil)
		defer memRes.Body.Close()
		testutil.RequireStatus(t, memRes, 200)
		var memResp struct {
			Members []map[string]any `json:"members"`
		}
		json.NewDecoder(memRes.Body).Decode(&memResp)
		found := false
		for _, m := range memResp.Members {
			if m["id"] == bob.UserID {
				found = true
			}
		}
		testutil.RequireTrue(t, found, "bob not in member list after join")
	})
}

func TestReactionErrors(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "rxerr@a.t", "RxAlice", "password123")
	bob := f.Register(t, "rxerr@b.t", "RxBob", "password123")

	var chatID, msgID string
	{
		res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
			"type": "group", "name": "RxnErr", "member_ids": []string{bob.UserID},
		})
		var c map[string]any
		json.NewDecoder(res.Body).Decode(&c)
		res.Body.Close()
		chatID = c["id"].(string)

		res2 := f.Do(t, "POST", "/api/chats/"+chatID+"/messages", alice.AccessToken, map[string]string{
			"content": "reaction test",
		})
		var m map[string]any
		json.NewDecoder(res2.Body).Decode(&m)
		res2.Body.Close()
		msgID = m["id"].(string)
	}

	t.Run("reaction on nonexistent message", func(t *testing.T) {
		res := f.Do(t, "PUT", "/api/chats/"+chatID+"/messages/nonexistent-id/reactions/%F0%9F%91%8D", alice.AccessToken, nil)
		defer res.Body.Close()
		testutil.RequireStatus(t, res, 404)
	})

	t.Run("non-member cannot react", func(t *testing.T) {
		carol := f.Register(t, "rxerr@c.t", "RxCarol", "password123")
		res := f.Do(t, "PUT", "/api/chats/"+chatID+"/messages/"+msgID+"/reactions/%F0%9F%91%8D", carol.AccessToken, nil)
		defer res.Body.Close()
		testutil.RequireStatus(t, res, 403)
	})

	t.Run("remove nonexistent reaction", func(t *testing.T) {
		res := f.Do(t, "DELETE", "/api/chats/"+chatID+"/messages/"+msgID+"/reactions/%E2%9D%A4", alice.AccessToken, nil)
		defer res.Body.Close()
		testutil.RequireStatusAny(t, res, 200, 400, 404)
	})
}

func TestSSEConnection(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "sse@t.t", "SSEUser", "password123")

	req, _ := http.NewRequest("GET", f.HTTP.URL+"/api/events", nil)
	req.Header.Set("Authorization", "Bearer "+alice.AccessToken)
	res, err := http.DefaultClient.Do(req)
	testutil.RequireNoError(t, err)
	defer res.Body.Close()
	testutil.RequireStatus(t, res, 200)
	testutil.RequireEqual(t, res.Header.Get("Content-Type"), "text/event-stream")

	scanner := bufio.NewScanner(res.Body)
	gotReady := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ready") {
			gotReady = true
			break
		}
		if strings.HasPrefix(line, "event: ") {
			break
		}
	}
	testutil.RequireTrue(t, gotReady, "did not receive ready event")
}

func TestSSEInvalidToken(t *testing.T) {
	f := testutil.New(t)

	res := f.Do(t, "GET", "/api/events", "invalid-jwt", nil)
	defer res.Body.Close()
	testutil.RequireStatus(t, res, 401)
}

func TestSSEMissingToken(t *testing.T) {
	f := testutil.New(t)

	res := f.Do(t, "GET", "/api/events", "", nil)
	defer res.Body.Close()
	testutil.RequireStatus(t, res, 401)
}

func TestCreateOrGetDM(t *testing.T) {
	f := testutil.New(t)
	a := f.Register(t, "dma@test.dev", "DMA", "testPass1!")
	b := f.Register(t, "dmb@test.dev", "DMB", "testPass1!")

	res := f.Do(t, "POST", "/api/dms", a.AccessToken, map[string]string{
		"user_id": b.UserID,
	})
	defer res.Body.Close()
	testutil.RequireStatusAny(t, res, 201, 200)

	res2 := f.Do(t, "POST", "/api/dms", a.AccessToken, map[string]string{
		"user_id": b.UserID,
	})
	defer res2.Body.Close()
	testutil.RequireStatus(t, res2, 200)

	res3 := f.Do(t, "POST", "/api/dms", a.AccessToken, map[string]string{
		"user_id": a.UserID,
	})
	defer res3.Body.Close()
	testutil.RequireTrue(t, res3.StatusCode != 201 && res3.StatusCode != 200, "self dm should fail")
}

func TestGetChat_AsMemberAndNonMember(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "gca@t.t", "GCA", "password123")
	bob := f.Register(t, "gcb@t.t", "GCB", "password123")

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "GetChatTest", "member_ids": []string{},
	})
	var chat struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	getRes := f.Do(t, "GET", "/api/chats/"+chat.ID, alice.AccessToken, nil)
	defer getRes.Body.Close()
	testutil.RequireStatus(t, getRes, 200)

	getRes2 := f.Do(t, "GET", "/api/chats/"+chat.ID, bob.AccessToken, nil)
	defer getRes2.Body.Close()
	testutil.RequireStatus(t, getRes2, 403)
}

func TestGetChat_NotFound(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "gcnf@t.t", "GCNF", "password123")
	res := f.Do(t, "GET", "/api/chats/nonexistent", alice.AccessToken, nil)
	defer res.Body.Close()
	testutil.RequireStatus(t, res, 403)
}

func TestRenameDelete_DMNotAllowed(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "rddm1@t.t", "RD_DM_A", "password123")
	bob := f.Register(t, "rddm2@t.t", "RD_DM_B", "password123")

	res := f.Do(t, "POST", "/api/dms", alice.AccessToken, map[string]string{
		"user_id": bob.UserID,
	})
	var dm struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&dm)
	res.Body.Close()

	t.Run("rename dm → 400", func(t *testing.T) {
		res2 := f.Do(t, "PATCH", "/api/chats/"+dm.ID, alice.AccessToken, map[string]string{
			"name": "new name",
		})
		defer res2.Body.Close()
		testutil.RequireStatus(t, res2, 400)
	})

	t.Run("delete dm → 400", func(t *testing.T) {
		res2 := f.Do(t, "DELETE", "/api/chats/"+dm.ID, alice.AccessToken, nil)
		defer res2.Body.Close()
		testutil.RequireStatus(t, res2, 400)
	})
}

func TestDeleteMessage_NonAuthor(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "dmn1@t.t", "DelMsgA", "password123")
	bob := f.Register(t, "dmn2@t.t", "DelMsgB", "password123")

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "DelMsgTest", "member_ids": []string{bob.UserID},
	})
	var chat struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	msgRes := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", bob.AccessToken, map[string]string{
		"content": "bob's msg",
	})
	var msg struct {
		ID string `json:"id"`
	}
	json.NewDecoder(msgRes.Body).Decode(&msg)
	msgRes.Body.Close()

	t.Run("non-owner member cannot delete other's message → 403", func(t *testing.T) {
		carol := f.Register(t, "dmn3@t.t", "DelMsgC", "password123")
		res2 := f.Do(t, "POST", "/api/chats/"+chat.ID+"/members", alice.AccessToken, map[string]string{
			"user_id": carol.UserID,
		})
		res2.Body.Close()

		delRes := f.Do(t, "DELETE", "/api/chats/"+chat.ID+"/messages/"+msg.ID, carol.AccessToken, nil)
		defer delRes.Body.Close()
		testutil.RequireStatus(t, delRes, 403)
	})

	t.Run("chat mismatch → 400", func(t *testing.T) {
		res2 := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
			"type": "group", "name": "OtherChat2", "member_ids": []string{},
		})
		var otherChat struct {
			ID string `json:"id"`
		}
		json.NewDecoder(res2.Body).Decode(&otherChat)
		res2.Body.Close()

		delRes := f.Do(t, "DELETE", "/api/chats/"+otherChat.ID+"/messages/"+msg.ID, alice.AccessToken, nil)
		defer delRes.Body.Close()
		testutil.RequireStatus(t, delRes, 400)
	})
}

func TestListMembers_NonMember(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "lm1@t.t", "ListMemA", "password123")
	bob := f.Register(t, "lm2@t.t", "ListMemB", "password123")

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "ListMemTest", "member_ids": []string{},
	})
	var chat struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	memRes := f.Do(t, "GET", "/api/chats/"+chat.ID+"/members", bob.AccessToken, nil)
	defer memRes.Body.Close()
	testutil.RequireStatus(t, memRes, 403)
}

func TestAddMember_DMAndDuplicate(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "adm1@t.t", "AddMemA", "password123")
	bob := f.Register(t, "adm2@t.t", "AddMemB", "password123")
	carol := f.Register(t, "adm3@t.t", "AddMemC", "password123")

	res := f.Do(t, "POST", "/api/dms", alice.AccessToken, map[string]string{
		"user_id": bob.UserID,
	})
	var dm struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&dm)
	res.Body.Close()

	t.Run("add member to dm → 400", func(t *testing.T) {
		res2 := f.Do(t, "POST", "/api/chats/"+dm.ID+"/members", alice.AccessToken, map[string]string{
			"user_id": carol.UserID,
		})
		defer res2.Body.Close()
		testutil.RequireStatus(t, res2, 400)
	})

	t.Run("add already member → 409", func(t *testing.T) {
		res3 := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
			"type": "group", "name": "AddDup", "member_ids": []string{bob.UserID},
		})
		var c struct {
			ID string `json:"id"`
		}
		json.NewDecoder(res3.Body).Decode(&c)
		res3.Body.Close()

		res4 := f.Do(t, "POST", "/api/chats/"+c.ID+"/members", alice.AccessToken, map[string]string{
			"user_id": bob.UserID,
		})
		defer res4.Body.Close()
		testutil.RequireStatus(t, res4, 409)
	})

	t.Run("add nonexistent user → 404", func(t *testing.T) {
		res5 := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
			"type": "group", "name": "AddNoUser", "member_ids": []string{},
		})
		var c2 struct {
			ID string `json:"id"`
		}
		json.NewDecoder(res5.Body).Decode(&c2)
		res5.Body.Close()

		res6 := f.Do(t, "POST", "/api/chats/"+c2.ID+"/members", alice.AccessToken, map[string]string{
			"user_id": "nonexistent-user-id",
		})
		defer res6.Body.Close()
		testutil.RequireStatus(t, res6, 404)
	})
}

func TestRemoveMember_DMAndOwner(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "rm1@t.t", "RemMemA", "password123")
	bob := f.Register(t, "rm2@t.t", "RemMemB", "password123")
	carol := f.Register(t, "rm3@t.t", "RemMemC", "password123")

	res := f.Do(t, "POST", "/api/dms", alice.AccessToken, map[string]string{
		"user_id": bob.UserID,
	})
	var dm struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&dm)
	res.Body.Close()

	t.Run("remove from dm → 400", func(t *testing.T) {
		res2 := f.Do(t, "DELETE", "/api/chats/"+dm.ID+"/members/"+bob.UserID, alice.AccessToken, nil)
		defer res2.Body.Close()
		testutil.RequireStatus(t, res2, 400)
	})

	t.Run("non-admin kick owner → 403", func(t *testing.T) {
		res3 := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
			"type": "group", "name": "KickOwner", "member_ids": []string{bob.UserID, carol.UserID},
		})
		var c struct {
			ID string `json:"id"`
		}
		json.NewDecoder(res3.Body).Decode(&c)
		res3.Body.Close()

		res4 := f.Do(t, "DELETE", "/api/chats/"+c.ID+"/members/"+alice.UserID, bob.AccessToken, nil)
		defer res4.Body.Close()
		testutil.RequireStatus(t, res4, 403)
	})
}

func TestSendMessage_BadJSON(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "smbj@t.t", "SmBadJSON", "password123")

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "BadJSON", "member_ids": []string{},
	})
	var chat struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	req, _ := http.NewRequest("POST", f.HTTP.URL+"/api/chats/"+chat.ID+"/messages", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+alice.AccessToken)
	res2, err := http.DefaultClient.Do(req)
	testutil.RequireNoError(t, err)
	defer res2.Body.Close()
	testutil.RequireStatus(t, res2, 400)
}

func TestMarkRead_NoBody(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "mrempty@t.t", "MrEmpty", "password123")

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "MarkReadEmpty", "member_ids": []string{},
	})
	var chat struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	readRes := f.Do(t, "POST", "/api/chats/"+chat.ID+"/read", alice.AccessToken, nil)
	defer readRes.Body.Close()
	testutil.RequireStatus(t, readRes, 200)
}

func TestUpdateMe_EmptyBody(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "emptyup@t.t", "EmptyUp", "password123")

	req, _ := http.NewRequest("PATCH", f.HTTP.URL+"/api/users/me", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+alice.AccessToken)
	res, err := http.DefaultClient.Do(req)
	testutil.RequireNoError(t, err)
	defer res.Body.Close()
	testutil.RequireStatus(t, res, 400)
}

func TestSendMessage_EmptyContentNoAttachments(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "emptymsgh@t.t", "EmptyMsgH", "password123")

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "EmptyMsgH", "member_ids": []string{},
	})
	var chat struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	res2 := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", alice.AccessToken, map[string]string{
		"content": "",
	})
	defer res2.Body.Close()
	testutil.RequireStatus(t, res2, 400)
}
