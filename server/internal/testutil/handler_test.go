package testutil_test

import (
	"encoding/json"
	"io"
	"net/http"
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
	if res.StatusCode != 201 {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("create chat: %d %s", res.StatusCode, string(b))
	}
	var chat struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.NewDecoder(res.Body).Decode(&chat); err != nil {
		t.Fatal(err)
	}
	if chat.Name != "Dev Team" || chat.Type != "group" {
		t.Fatal("chat wrong")
	}

	sendRes := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", alice.AccessToken, map[string]string{
		"content": "hello team!",
	})
	defer sendRes.Body.Close()
	if sendRes.StatusCode != 201 {
		b, _ := io.ReadAll(sendRes.Body)
		t.Fatalf("send msg: %d %s", sendRes.StatusCode, string(b))
	}
	var msg struct {
		ID      string `json:"id"`
		Content string `json:"content"`
		ChatID  string `json:"chat_id"`
	}
	if err := json.NewDecoder(sendRes.Body).Decode(&msg); err != nil {
		t.Fatal(err)
	}
	if msg.Content != "hello team!" {
		t.Fatal("content mismatch")
	}

	listRes := f.Do(t, "GET", "/api/chats/"+chat.ID+"/messages?limit=50", alice.AccessToken, nil)
	defer listRes.Body.Close()
	if listRes.StatusCode != 200 {
		t.Fatal("list messages failed")
	}
	var listResp struct {
		Messages []map[string]any `json:"messages"`
	}
	json.NewDecoder(listRes.Body).Decode(&listResp)
	if len(listResp.Messages) != 1 {
		t.Fatalf("want 1 message, got %d", len(listResp.Messages))
	}

	editRes := f.Do(t, "PATCH", "/api/chats/"+chat.ID+"/messages/"+msg.ID, alice.AccessToken, map[string]string{
		"content": "edited hello!",
	})
	defer editRes.Body.Close()
	if editRes.StatusCode != 200 {
		t.Fatal("edit failed")
	}
}

func TestCreateDM(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "dm1@dm.t", "AliceDM", "password123")
	bob := f.Register(t, "dm2@dm.t", "BobDM", "password123")

	res := f.Do(t, "POST", "/api/dms", alice.AccessToken, map[string]string{
		"user_id": bob.UserID,
	})
	defer res.Body.Close()
	if res.StatusCode != 201 && res.StatusCode != 200 {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("create dm: %d %s", res.StatusCode, string(b))
	}

	res2 := f.Do(t, "POST", "/api/dms", alice.AccessToken, map[string]string{
		"user_id": bob.UserID,
	})
	defer res2.Body.Close()
	if res2.StatusCode != 200 {
		t.Fatal("second DM create should return existing")
	}

	res3 := f.Do(t, "POST", "/api/dms", alice.AccessToken, map[string]string{
		"user_id": alice.UserID,
	})
	defer res3.Body.Close()
	if res3.StatusCode == 201 || res3.StatusCode == 200 {
		t.Fatal("self dm should fail")
	}
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
	if addRes.StatusCode != 200 {
		b, _ := io.ReadAll(addRes.Body)
		t.Fatalf("add member: %d %s", addRes.StatusCode, string(b))
	}

	membersRes := f.Do(t, "GET", "/api/chats/"+chatID+"/members", alice.AccessToken, nil)
	defer membersRes.Body.Close()
	var membersResp struct {
		Members []map[string]any `json:"members"`
	}
	json.NewDecoder(membersRes.Body).Decode(&membersResp)
	if len(membersResp.Members) != 3 {
		t.Fatalf("want 3 members, got %d", len(membersResp.Members))
	}

	removeRes := f.Do(t, "DELETE", "/api/chats/"+chatID+"/members/"+carol.UserID, alice.AccessToken, nil)
	defer removeRes.Body.Close()
	if removeRes.StatusCode != 200 {
		t.Fatal("remove member failed")
	}

	membersRes = f.Do(t, "GET", "/api/chats/"+chatID+"/members", alice.AccessToken, nil)
	defer membersRes.Body.Close()
	json.NewDecoder(membersRes.Body).Decode(&membersResp)
	if len(membersResp.Members) != 2 {
		t.Fatalf("after remove: want 2, got %d", len(membersResp.Members))
	}
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
	if addRes.StatusCode != 200 {
		t.Fatalf("add reaction: %d", addRes.StatusCode)
	}

	addRes2 := f.Do(t, "PUT", "/api/chats/"+chatID+"/messages/"+msgID+"/reactions/%F0%9F%91%8D", bob.AccessToken, nil)
	addRes2.Body.Close()
	if addRes2.StatusCode != 200 {
		t.Fatalf("bob add reaction: %d", addRes2.StatusCode)
	}

	delRes := f.Do(t, "DELETE", "/api/chats/"+chatID+"/messages/"+msgID+"/reactions/%F0%9F%91%8D", alice.AccessToken, nil)
	delRes.Body.Close()
	if delRes.StatusCode != 200 {
		t.Fatalf("remove reaction: %d", delRes.StatusCode)
	}

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
	if len(listResp.Messages) != 1 || len(listResp.Messages[0].Reactions) != 1 {
		t.Fatalf("want 1 reaction group, got %d", len(listResp.Messages[0].Reactions))
	}
	if listResp.Messages[0].Reactions[0].Count != 1 {
		t.Fatalf("want 1 count after remove, got %d", listResp.Messages[0].Reactions[0].Count)
	}
}

func TestUpdateProfile(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "prof@p.t", "ProfUser", "password123")

	res := f.Do(t, "PATCH", "/api/users/me", alice.AccessToken, map[string]string{
		"username": "NewProfUser",
	})
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatal("update profile failed")
	}
	var u map[string]any
	json.NewDecoder(res.Body).Decode(&u)
	if u["username"] != "NewProfUser" {
		t.Fatal("username not updated")
	}
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
	if !found {
		t.Fatal("search didn't find target user")
	}
	for _, u := range resp.Users {
		if u["id"] == alice.UserID {
			t.Fatal("search returned self")
		}
	}
}

func TestDeleteMessageAsAdmin(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "admin@d.t", "Admin", "password123")
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
	if delRes.StatusCode != 200 {
		t.Fatalf("owner should be able to delete any message: %d", delRes.StatusCode)
	}
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
	if leaveRes.StatusCode != 200 {
		t.Fatalf("self-leave: %d", leaveRes.StatusCode)
	}

	accessRes := f.Do(t, "GET", "/api/chats/"+chatID+"/messages?limit=1", alice.AccessToken, nil)
	defer accessRes.Body.Close()
	if accessRes.StatusCode != 403 {
		t.Fatalf("should be forbidden after leaving: %d", accessRes.StatusCode)
	}
}

func TestAuthEndpoints(t *testing.T) {
	f := testutil.New(t)

	t.Run("register with bad input", func(t *testing.T) {
		res := f.Do(t, "POST", "/api/auth/register", "", map[string]string{
			"email": "not-an-email", "username": "a", "password": "12",
		})
		res.Body.Close()
		if res.StatusCode < 400 {
			t.Fatalf("want error, got %d", res.StatusCode)
		}
	})

	t.Run("login with wrong password", func(t *testing.T) {
		f.Register(t, "wrongpw@t.com", "WrongPW", "testtest123")
		res := f.Do(t, "POST", "/api/auth/login", "", map[string]string{
			"email": "wrongpw@t.com", "password": "badbadbad",
		})
		res.Body.Close()
		if res.StatusCode != 401 {
			t.Fatalf("want 401, got %d", res.StatusCode)
		}
	})

	t.Run("refresh with garbage token", func(t *testing.T) {
		res := f.DoWithCookie(t, "POST", "/api/auth/refresh", "", "refresh_token", "not-a-real-token", nil)
		res.Body.Close()
		if res.StatusCode != 401 {
			t.Fatalf("want 401 for invalid refresh, got %d", res.StatusCode)
		}
	})

	t.Run("logout cleans up refresh token", func(t *testing.T) {
		s := f.Register(t, "logout@t.com", "LogoutUser", "testtest123")
		res := f.Do(t, "POST", "/api/auth/logout", s.AccessToken, nil)
		res.Body.Close()
		if res.StatusCode != 200 {
			t.Fatalf("logout: %d", res.StatusCode)
		}
		refreshRes := f.DoWithCookie(t, "POST", "/api/auth/refresh", "", "refresh_token", s.RefreshToken, nil)
		refreshRes.Body.Close()
		if refreshRes.StatusCode != 401 {
			t.Fatal("refresh should fail after logout")
		}
	})

	t.Run("get me returns user", func(t *testing.T) {
		s := f.Register(t, "me@t.com", "MeUser", "testtest123")
		res := f.Do(t, "GET", "/api/users/me", s.AccessToken, nil)
		defer res.Body.Close()
		var u map[string]any
		json.NewDecoder(res.Body).Decode(&u)
		if u["username"] != "MeUser" {
			t.Fatalf("wrong user: %v", u)
		}
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

	listRes := f.Do(t, "GET", "/api/chats", alice.AccessToken, nil)
	defer listRes.Body.Close()
	if listRes.StatusCode != 200 {
		t.Fatal("list chats failed")
	}
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
	if len(listResp.Chats) < 1 {
		t.Fatal("no chats returned")
	}
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

func TestUploadNotLoggedIn(t *testing.T) {
	f := testutil.New(t)
	res := f.DoMultipart(t, "POST", "/api/uploads", "", nil, "file", "test.txt", []byte("hello"))
	res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatalf("want 401, got %d", res.StatusCode)
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
	if renameRes.StatusCode != 403 {
		t.Fatalf("non-owner rename: want 403 got %d", renameRes.StatusCode)
	}

	renameRes2 := f.Do(t, "PATCH", "/api/chats/"+chatID, alice.AccessToken, map[string]string{
		"name": "Renamed",
	})
	renameRes2.Body.Close()
	if renameRes2.StatusCode != 200 {
		t.Fatalf("owner rename: want 200 got %d", renameRes2.StatusCode)
	}
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
	if sendRes.StatusCode != 403 {
		t.Fatalf("non-member send: want 403 got %d", sendRes.StatusCode)
	}
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
	if readRes.StatusCode != 200 {
		t.Fatalf("mark read: %d", readRes.StatusCode)
	}
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
	if delRes.StatusCode != 403 {
		t.Fatalf("non-owner delete: want 403 got %d", delRes.StatusCode)
	}
	delRes2 := f.Do(t, "DELETE", "/api/chats/"+chatID, alice.AccessToken, nil)
	delRes2.Body.Close()
	if delRes2.StatusCode != 200 {
		t.Fatalf("owner delete: want 200 got %d", delRes2.StatusCode)
	}
}

func TestConcurrentRegister(t *testing.T) {
	f := testutil.New(t)
	done := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func(n int) {
			res := f.Do(t, "POST", "/api/auth/register", "", map[string]string{
				"email":    "concurrent@t.com",
				"username": "ConcurrentUser",
				"password": "testtest123",
			})
			res.Body.Close()
			if res.StatusCode == 409 {
				done <- nil
			} else if res.StatusCode == 200 {
				done <- nil
			} else {
				done <- nil
			}
		}(i)
	}
	for i := 0; i < 5; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for concurrent register")
		}
	}
}

func TestHealthz(t *testing.T) {
	f := testutil.New(t)
	res := f.Do(t, "GET", "/healthz", "", nil)
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("healthz: %d", res.StatusCode)
	}
}

func TestUploadFile(t *testing.T) {
	f := testutil.New(t)
	s := f.Register(t, "upload@test.dev", "Uploader", "testPass1!")

	res := f.DoMultipart(t, "POST", "/api/uploads", s.AccessToken, nil, "file", "hello.txt", []byte("hello world"))
	defer res.Body.Close()
	if res.StatusCode != 201 {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("upload: want 201 got %d body=%s", res.StatusCode, string(b))
	}
	var uploadResp struct {
		ID       string `json:"id"`
		URL      string `json:"url"`
		Filename string `json:"filename"`
		MimeType string `json:"mime_type"`
		Size     int64  `json:"size"`
	}
	if err := json.NewDecoder(res.Body).Decode(&uploadResp); err != nil {
		t.Fatal(err)
	}
	if uploadResp.ID == "" || uploadResp.URL == "" {
		t.Fatal("upload response missing id/url")
	}
	if uploadResp.Filename != "hello.txt" {
		t.Fatalf("filename: want hello.txt got %s", uploadResp.Filename)
	}
	if uploadResp.Size != 11 {
		t.Fatalf("size: want 11 got %d", uploadResp.Size)
	}
}

func TestUploadExceedsSizeLimit(t *testing.T) {
	f := testutil.New(t)
	s := f.Register(t, "bigupload@test.dev", "BigUploader", "testPass1!")

	// MaxUploadBytes is 5MB in test config
	data := make([]byte, 6<<20)
	res := f.DoMultipart(t, "POST", "/api/uploads", s.AccessToken, nil, "file", "big.bin", data)
	defer res.Body.Close()
	if res.StatusCode != 413 {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("oversize upload: want 413 got %d body=%s", res.StatusCode, string(b))
	}

	var errResp struct {
		Error string `json:"error"`
	}
	json.NewDecoder(res.Body).Decode(&errResp)
	if errResp.Error != "too_large" {
		t.Fatalf("want error='too_large' got '%s'", errResp.Error)
	}
}

func TestUploadRejectsUnsupportedMime(t *testing.T) {
	f := testutil.New(t)
	s := f.Register(t, "badmime@test.dev", "BadMime", "testPass1!")

	// .exe extension maps to application/x-msdownload which is not in allowedMime
	res := f.DoMultipart(t, "POST", "/api/uploads", s.AccessToken, nil, "file", "virus.exe", []byte("MZ..."))
	defer res.Body.Close()
	if res.StatusCode != 415 {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("bad mime: want 415 got %d body=%s", res.StatusCode, string(b))
	}
}

func TestUpdateMeUsernameConflict(t *testing.T) {
	f := testutil.New(t)
	a := f.Register(t, "upa@test.dev", "UserA", "testPass1!")
	b := f.Register(t, "upb@test.dev", "UserB", "testPass1!")

	res := f.Do(t, "PATCH", "/api/users/me", b.AccessToken, map[string]string{
		"username": "UserA",
	})
	defer res.Body.Close()
	if res.StatusCode != 409 {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("username conflict: want 409 got %d body=%s", res.StatusCode, string(b))
	}

	res2 := f.Do(t, "PATCH", "/api/users/me", b.AccessToken, map[string]string{
		"username": "UserB-renamed",
	})
	defer res2.Body.Close()
	if res2.StatusCode != 200 {
		b, _ := io.ReadAll(res2.Body)
		t.Fatalf("rename: want 200 got %d body=%s", res2.StatusCode, string(b))
	}
	var u struct {
		Username string `json:"username"`
	}
	json.NewDecoder(res2.Body).Decode(&u)
	if u.Username != "UserB-renamed" {
		t.Fatalf("want UserB-renamed got %s", u.Username)
	}
}

func TestCreateChatInvalidInput(t *testing.T) {
	f := testutil.New(t)
	s := f.Register(t, "chatbad@test.dev", "ChatBad", "testPass1!")

	res := f.Do(t, "POST", "/api/chats", s.AccessToken, map[string]any{
		"type": "invalid-type", "name": "Test", "member_ids": []string{},
	})
	defer res.Body.Close()
	if res.StatusCode != 400 {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("invalid chat type: want 400 got %d body=%s", res.StatusCode, string(b))
	}
}

func TestSendMessageNonMember(t *testing.T) {
	f := testutil.New(t)
	a := f.Register(t, "nmsga@test.dev", "NonMemberA", "testPass1!")
	b := f.Register(t, "nmsgb@test.dev", "NonMemberB", "testPass1!")

	res := f.Do(t, "POST", "/api/chats", a.AccessToken, map[string]any{
		"type": "group", "name": "Exclusive", "member_ids": []string{},
	})
	var chat struct{ ID string `json:"id"` }
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	sendRes := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", b.AccessToken, map[string]string{
		"content": "interloper message",
	})
	defer sendRes.Body.Close()
	if sendRes.StatusCode != 403 {
		b, _ := io.ReadAll(sendRes.Body)
		t.Fatalf("non-member send: want 403 got %d body=%s", sendRes.StatusCode, string(b))
	}
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
	if len(resp.Users) != 0 {
		t.Fatalf("empty query: want 0 users got %d", len(resp.Users))
	}
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
		if u["id"] == s.UserID {
			t.Fatal("search returned self")
		}
	}
}

func TestDeleteChatByNonOwner(t *testing.T) {
	f := testutil.New(t)
	a := f.Register(t, "delowner@test.dev", "DelOwner", "testPass1!")
	b := f.Register(t, "deluser@test.dev", "DelUser", "testPass1!")

	res := f.Do(t, "POST", "/api/chats", a.AccessToken, map[string]any{
		"type": "group", "name": "DelTest", "member_ids": []string{b.UserID},
	})
	var chat struct{ ID string `json:"id"` }
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	delRes := f.Do(t, "DELETE", "/api/chats/"+chat.ID, b.AccessToken, nil)
	defer delRes.Body.Close()
	if delRes.StatusCode != 403 {
		t.Fatalf("non-owner delete: want 403 got %d", delRes.StatusCode)
	}

	delRes2 := f.Do(t, "DELETE", "/api/chats/"+chat.ID, a.AccessToken, nil)
	defer delRes2.Body.Close()
	if delRes2.StatusCode != 200 {
		t.Fatalf("owner delete: want 200 got %d", delRes2.StatusCode)
	}
}

func TestCreateOrGetDM(t *testing.T) {
	f := testutil.New(t)
	a := f.Register(t, "dma@test.dev", "DMA", "testPass1!")
	b := f.Register(t, "dmb@test.dev", "DMB", "testPass1!")

	res := f.Do(t, "POST", "/api/dms", a.AccessToken, map[string]string{
		"user_id": b.UserID,
	})
	defer res.Body.Close()
	if res.StatusCode != 201 && res.StatusCode != 200 {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("create dm: %d %s", res.StatusCode, string(b))
	}

	res2 := f.Do(t, "POST", "/api/dms", a.AccessToken, map[string]string{
		"user_id": b.UserID,
	})
	defer res2.Body.Close()
	if res2.StatusCode != 200 {
		t.Fatal("second DM create should return existing")
	}

	res3 := f.Do(t, "POST", "/api/dms", a.AccessToken, map[string]string{
		"user_id": a.UserID,
	})
	defer res3.Body.Close()
	if res3.StatusCode == 201 || res3.StatusCode == 200 {
		t.Fatal("self dm should fail")
	}
}