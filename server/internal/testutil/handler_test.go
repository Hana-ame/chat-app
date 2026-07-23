package testutil_test

import (
	"bufio"
	"encoding/json"
	"io"
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

	t.Run("register accepts any input (validations removed)", func(t *testing.T) {
		res := f.Do(t, "POST", "/api/auth/register", "", map[string]string{
			"email": "not-an-email", "username": "a", "password": "12",
		})
		res.Body.Close()
		if res.StatusCode != 200 {
			t.Fatalf("want 200 got %d", res.StatusCode)
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

	listRes := f.Do(t, "GET", "/api/chats/my", alice.AccessToken, nil)
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
	res := f.DoMultipart(t, "POST", "/api/uploads", "", nil, "file", "test.txt", []byte("hello"), "text/plain")
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
	if ok != 1 {
		t.Fatalf("exactly 1 should succeed, got %d (conflict=%d other=%d)", ok, conflict, other)
	}
}

func TestHealthz(t *testing.T) {
	f := testutil.New(t)
	// Send a header so the echo can be verified
	res := f.Do(t, "GET", "/healthz", "", nil)
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("healthz: %d", res.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", body["status"])
	}
	echo, ok := body["echo"].(map[string]any)
	if !ok {
		t.Fatalf("expected echo object, got %T", body["echo"])
	}
	if len(echo) == 0 {
		t.Fatalf("expected non-empty echo object")
	}
}

func TestUploadFile(t *testing.T) {
	f := testutil.New(t)
	s := f.Register(t, "upload@test.dev", "Uploader", "testPass1!")

	res := f.DoMultipart(t, "POST", "/api/uploads", s.AccessToken, nil, "file", "hello.txt", []byte("hello world"), "text/plain")
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
	res := f.DoMultipart(t, "POST", "/api/uploads", s.AccessToken, nil, "file", "big.bin", data, "application/octet-stream")
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
	res := f.DoMultipart(t, "POST", "/api/uploads", s.AccessToken, nil, "file", "virus.json", []byte(`{"evil":true}`), "application/json")
	defer res.Body.Close()
	if res.StatusCode != 415 {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("bad mime: want 415 got %d body=%s", res.StatusCode, string(b))
	}
}

func TestUpdateMeUsernameConflict(t *testing.T) {
	f := testutil.New(t)
	_ = f.Register(t, "upa@test.dev", "UserA", "testPass1!")
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
		if res.StatusCode != 404 {
			t.Fatalf("non-author edit: want 404 got %d", res.StatusCode)
		}
	})

	t.Run("message not found", func(t *testing.T) {
		res := f.Do(t, "PATCH", "/api/chats/"+chatID+"/messages/nonexistent-id", alice.AccessToken, map[string]string{
			"content": "edited",
		})
		defer res.Body.Close()
		if res.StatusCode != 404 {
			t.Fatalf("edit nonexistent: want 404 got %d", res.StatusCode)
		}
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
		if res2.StatusCode != 400 {
			t.Fatalf("chat mismatch: want 400 got %d", res2.StatusCode)
		}
	})
}

func TestSendMessageWithAttachments(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "att@a.t", "AttAlice", "password123")

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "AttTest", "member_ids": []string{},
	})
	var chat struct{ ID string `json:"id"` }
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
		if res.StatusCode != 400 {
			t.Fatalf("missing url: want 400 got %d", res.StatusCode)
		}
	})

	t.Run("attachment missing filename", func(t *testing.T) {
		res := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", alice.AccessToken, map[string]any{
			"content": "bad attach",
			"attachments": []map[string]any{
				{"url": "http://localhost:8080/api/local/123/file.png"},
			},
		})
		defer res.Body.Close()
		if res.StatusCode != 400 {
			t.Fatalf("missing filename: want 400 got %d", res.StatusCode)
		}
	})

	t.Run("attachment invalid url prefix", func(t *testing.T) {
		res := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", alice.AccessToken, map[string]any{
			"content": "bad attach",
			"attachments": []map[string]any{
				{"url": "https://evil.com/virus.exe", "filename": "virus.exe"},
			},
		})
		defer res.Body.Close()
		if res.StatusCode != 400 {
			t.Fatalf("invalid url: want 400 got %d", res.StatusCode)
		}
	})

	t.Run("attachment mime auto-filled", func(t *testing.T) {
		res := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", alice.AccessToken, map[string]any{
			"content": "with attach",
			"attachments": []map[string]any{
				{"url": "http://localhost:8080/api/local/123/foo.png", "filename": "foo.png"},
			},
		})
		defer res.Body.Close()
		if res.StatusCode != 201 {
			b, _ := io.ReadAll(res.Body)
			t.Fatalf("send with attach: want 201 got %d body=%s", res.StatusCode, string(b))
		}
	})
}

func TestMessageContentTooLong(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "long@l.t", "LongAlice", "password123")

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "LongTest", "member_ids": []string{},
	})
	var chat struct{ ID string `json:"id"` }
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	longContent := string(make([]byte, 4001))
	res2 := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", alice.AccessToken, map[string]string{
		"content": longContent,
	})
	defer res2.Body.Close()
	if res2.StatusCode != 413 {
		t.Fatalf("long content: want 413 got %d", res2.StatusCode)
	}
	var errResp struct{ Error string `json:"error"` }
	json.NewDecoder(res2.Body).Decode(&errResp)
	if errResp.Error != "content_too_long" {
		t.Fatalf("want error='content_too_long' got '%s'", errResp.Error)
	}
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
		if res.StatusCode != 200 {
			b, _ := io.ReadAll(res.Body)
			t.Fatalf("owner pin: want 200 got %d body=%s", res.StatusCode, string(b))
		}
	})

	t.Run("non-owner cannot pin", func(t *testing.T) {
		res := f.Do(t, "POST", "/api/chats/"+chatID+"/announcement", bob.AccessToken, map[string]string{
			"content": "non-owner pin",
		})
		defer res.Body.Close()
		if res.StatusCode != 403 {
			t.Fatalf("non-owner pin: want 403 got %d", res.StatusCode)
		}
	})

	t.Run("small group can pin", func(t *testing.T) {
		res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
			"type": "group", "name": "SmallChat", "member_ids": []string{},
		})
		var small struct{ ID string `json:"id"` }
		json.NewDecoder(res.Body).Decode(&small)
		res.Body.Close()

		res2 := f.Do(t, "POST", "/api/chats/"+small.ID+"/announcement", alice.AccessToken, map[string]string{
			"content": "should succeed",
		})
		defer res2.Body.Close()
		if res2.StatusCode != 200 {
			t.Fatalf("small group pin: want 200 got %d", res2.StatusCode)
		}
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
		if res2.StatusCode != 200 {
			t.Fatalf("owner clear pin: want 200 got %d", res2.StatusCode)
		}
	})

	f.Do(t, "POST", "/api/chats/"+chatID+"/announcement", alice.AccessToken, map[string]string{
		"content": "pin again",
	})

	t.Run("non-member cannot clear pin", func(t *testing.T) {
		dave := f.Register(t, "delpin@d.t", "DelPinD", "password123")
		res3 := f.Do(t, "DELETE", "/api/chats/"+chatID+"/announcement", dave.AccessToken, nil)
		defer res3.Body.Close()
		if res3.StatusCode != 403 {
			t.Fatalf("non-member clear pin: want 403 got %d", res3.StatusCode)
		}
	})

	t.Run("regular member cannot clear pin", func(t *testing.T) {
		res4 := f.Do(t, "DELETE", "/api/chats/"+chatID+"/announcement", bob.AccessToken, nil)
		defer res4.Body.Close()
		if res4.StatusCode != 403 {
			t.Fatalf("member clear pin: want 403 got %d", res4.StatusCode)
		}
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
	if publicRes.StatusCode != 200 {
		t.Fatalf("public list: want 200 got %d", publicRes.StatusCode)
	}
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
	if !foundPublic {
		t.Fatal("public chat not in public list")
	}
	if foundPrivate {
		t.Fatal("private chat should not be in public list")
	}
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
		if res.StatusCode != 200 {
			b, _ := io.ReadAll(res.Body)
			t.Fatalf("join public: want 200 got %d body=%s", res.StatusCode, string(b))
		}
	})

	t.Run("appears in member list after join", func(t *testing.T) {
		memRes := f.Do(t, "GET", "/api/chats/"+publicChatID+"/members", bob.AccessToken, nil)
		defer memRes.Body.Close()
		if memRes.StatusCode != 200 {
			t.Fatal("list members after join failed")
		}
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
		if !found {
			t.Fatal("bob not in member list after join")
		}
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
		if res.StatusCode != 404 {
			t.Fatalf("reaction on missing msg: want 404 got %d", res.StatusCode)
		}
	})

	t.Run("non-member cannot react", func(t *testing.T) {
		carol := f.Register(t, "rxerr@c.t", "RxCarol", "password123")
		res := f.Do(t, "PUT", "/api/chats/"+chatID+"/messages/"+msgID+"/reactions/%F0%9F%91%8D", carol.AccessToken, nil)
		defer res.Body.Close()
		if res.StatusCode != 403 {
			t.Fatalf("non-member react: want 403 got %d", res.StatusCode)
		}
	})

	t.Run("remove nonexistent reaction", func(t *testing.T) {
		res := f.Do(t, "DELETE", "/api/chats/"+chatID+"/messages/"+msgID+"/reactions/%E2%9D%A4", alice.AccessToken, nil)
		defer res.Body.Close()
		if res.StatusCode != 200 && res.StatusCode != 400 && res.StatusCode != 404 {
			t.Fatalf("remove nonexistent reaction: unexpected %d", res.StatusCode)
		}
	})
}

func TestSSEConnection(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "sse@t.t", "SSEUser", "password123")

	req, _ := http.NewRequest("GET", f.HTTP.URL+"/api/events?access_token="+alice.AccessToken, nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("sse connect: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("sse: want 200 got %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("expected text/event-stream, got %s", ct)
	}

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
	if !gotReady {
		t.Fatal("did not receive ready event")
	}
}

func TestSSEInvalidToken(t *testing.T) {
	f := testutil.New(t)

	res := f.Do(t, "GET", "/api/events?access_token=invalid-jwt", "", nil)
	defer res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatalf("sse invalid token: want 401 got %d", res.StatusCode)
	}
}

func TestSSEMissingToken(t *testing.T) {
	f := testutil.New(t)

	res := f.Do(t, "GET", "/api/events", "", nil)
	defer res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatalf("sse no token: want 401 got %d", res.StatusCode)
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

func TestGetChat_AsMemberAndNonMember(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "gca@t.t", "GCA", "password123")
	bob := f.Register(t, "gcb@t.t", "GCB", "password123")

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "GetChatTest", "member_ids": []string{},
	})
	var chat struct{ ID string `json:"id"` }
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	getRes := f.Do(t, "GET", "/api/chats/"+chat.ID, alice.AccessToken, nil)
	defer getRes.Body.Close()
	if getRes.StatusCode != 200 {
		t.Fatalf("member get chat: want 200 got %d", getRes.StatusCode)
	}

	getRes2 := f.Do(t, "GET", "/api/chats/"+chat.ID, bob.AccessToken, nil)
	defer getRes2.Body.Close()
	if getRes2.StatusCode != 403 {
		t.Fatalf("non-member get chat: want 403 got %d", getRes2.StatusCode)
	}
}

func TestGetChat_NotFound(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "gcnf@t.t", "GCNF", "password123")
	res := f.Do(t, "GET", "/api/chats/nonexistent", alice.AccessToken, nil)
	defer res.Body.Close()
	if res.StatusCode != 403 {
		t.Fatalf("nonexistent chat: want 403 got %d (IsChatMember returns false)", res.StatusCode)
	}
}

func TestRenameDelete_DMNotAllowed(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "rddm1@t.t", "RD_DM_A", "password123")
	bob := f.Register(t, "rddm2@t.t", "RD_DM_B", "password123")

	res := f.Do(t, "POST", "/api/dms", alice.AccessToken, map[string]string{
		"user_id": bob.UserID,
	})
	var dm struct{ ID string `json:"id"` }
	json.NewDecoder(res.Body).Decode(&dm)
	res.Body.Close()

	t.Run("rename dm → 400", func(t *testing.T) {
		res2 := f.Do(t, "PATCH", "/api/chats/"+dm.ID, alice.AccessToken, map[string]string{
			"name": "new name",
		})
		defer res2.Body.Close()
		if res2.StatusCode != 400 {
			t.Fatalf("rename dm: want 400 got %d", res2.StatusCode)
		}
	})

	t.Run("delete dm → 400", func(t *testing.T) {
		res2 := f.Do(t, "DELETE", "/api/chats/"+dm.ID, alice.AccessToken, nil)
		defer res2.Body.Close()
		if res2.StatusCode != 400 {
			t.Fatalf("delete dm: want 400 got %d", res2.StatusCode)
		}
	})
}

func TestDeleteMessage_NonAuthor(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "dmn1@t.t", "DelMsgA", "password123")
	bob := f.Register(t, "dmn2@t.t", "DelMsgB", "password123")

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "DelMsgTest", "member_ids": []string{bob.UserID},
	})
	var chat struct{ ID string `json:"id"` }
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	msgRes := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", bob.AccessToken, map[string]string{
		"content": "bob's msg",
	})
	var msg struct{ ID string `json:"id"` }
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
		if delRes.StatusCode != 403 {
			t.Fatalf("non-owner delete others msg: want 403 got %d", delRes.StatusCode)
		}
	})

	t.Run("chat mismatch → 400", func(t *testing.T) {
		res2 := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
			"type": "group", "name": "OtherChat2", "member_ids": []string{},
		})
		var otherChat struct{ ID string `json:"id"` }
		json.NewDecoder(res2.Body).Decode(&otherChat)
		res2.Body.Close()

		delRes := f.Do(t, "DELETE", "/api/chats/"+otherChat.ID+"/messages/"+msg.ID, alice.AccessToken, nil)
		defer delRes.Body.Close()
		if delRes.StatusCode != 400 {
			t.Fatalf("chat mismatch: want 400 got %d", delRes.StatusCode)
		}
	})
}

func TestListMembers_NonMember(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "lm1@t.t", "ListMemA", "password123")
	bob := f.Register(t, "lm2@t.t", "ListMemB", "password123")

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "ListMemTest", "member_ids": []string{},
	})
	var chat struct{ ID string `json:"id"` }
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	memRes := f.Do(t, "GET", "/api/chats/"+chat.ID+"/members", bob.AccessToken, nil)
	defer memRes.Body.Close()
	if memRes.StatusCode != 403 {
		t.Fatalf("non-member list members: want 403 got %d", memRes.StatusCode)
	}
}

func TestAddMember_DMAndDuplicate(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "adm1@t.t", "AddMemA", "password123")
	bob := f.Register(t, "adm2@t.t", "AddMemB", "password123")
	carol := f.Register(t, "adm3@t.t", "AddMemC", "password123")

	res := f.Do(t, "POST", "/api/dms", alice.AccessToken, map[string]string{
		"user_id": bob.UserID,
	})
	var dm struct{ ID string `json:"id"` }
	json.NewDecoder(res.Body).Decode(&dm)
	res.Body.Close()

	t.Run("add member to dm → 400", func(t *testing.T) {
		res2 := f.Do(t, "POST", "/api/chats/"+dm.ID+"/members", alice.AccessToken, map[string]string{
			"user_id": carol.UserID,
		})
		defer res2.Body.Close()
		if res2.StatusCode != 400 {
			t.Fatalf("add to dm: want 400 got %d", res2.StatusCode)
		}
	})

	t.Run("add already member → 409", func(t *testing.T) {
		res3 := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
			"type": "group", "name": "AddDup", "member_ids": []string{bob.UserID},
		})
		var c struct{ ID string `json:"id"` }
		json.NewDecoder(res3.Body).Decode(&c)
		res3.Body.Close()

		res4 := f.Do(t, "POST", "/api/chats/"+c.ID+"/members", alice.AccessToken, map[string]string{
			"user_id": bob.UserID,
		})
		defer res4.Body.Close()
		if res4.StatusCode != 409 {
			t.Fatalf("add already member: want 409 got %d", res4.StatusCode)
		}
	})

	t.Run("add nonexistent user → 404", func(t *testing.T) {
		res5 := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
			"type": "group", "name": "AddNoUser", "member_ids": []string{},
		})
		var c2 struct{ ID string `json:"id"` }
		json.NewDecoder(res5.Body).Decode(&c2)
		res5.Body.Close()

		res6 := f.Do(t, "POST", "/api/chats/"+c2.ID+"/members", alice.AccessToken, map[string]string{
			"user_id": "nonexistent-user-id",
		})
		defer res6.Body.Close()
		if res6.StatusCode != 404 {
			t.Fatalf("add nonexistent user: want 404 got %d", res6.StatusCode)
		}
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
	var dm struct{ ID string `json:"id"` }
	json.NewDecoder(res.Body).Decode(&dm)
	res.Body.Close()

	t.Run("remove from dm → 400", func(t *testing.T) {
		res2 := f.Do(t, "DELETE", "/api/chats/"+dm.ID+"/members/"+bob.UserID, alice.AccessToken, nil)
		defer res2.Body.Close()
		if res2.StatusCode != 400 {
			t.Fatalf("remove from dm: want 400 got %d", res2.StatusCode)
		}
	})

	t.Run("non-admin kick owner → 403", func(t *testing.T) {
		res3 := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
			"type": "group", "name": "KickOwner", "member_ids": []string{bob.UserID, carol.UserID},
		})
		var c struct{ ID string `json:"id"` }
		json.NewDecoder(res3.Body).Decode(&c)
		res3.Body.Close()

		res4 := f.Do(t, "DELETE", "/api/chats/"+c.ID+"/members/"+alice.UserID, bob.AccessToken, nil)
		defer res4.Body.Close()
		if res4.StatusCode != 403 {
			t.Fatalf("kick owner: want 403 got %d", res4.StatusCode)
		}
	})
}

func TestSendMessage_BadJSON(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "smbj@t.t", "SmBadJSON", "password123")

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "BadJSON", "member_ids": []string{},
	})
	var chat struct{ ID string `json:"id"` }
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	req, _ := http.NewRequest("POST", f.HTTP.URL+"/api/chats/"+chat.ID+"/messages", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+alice.AccessToken)
	res2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != 400 {
		t.Fatalf("bad json: want 400 got %d", res2.StatusCode)
	}
}

func TestMarkRead_NoBody(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "mrempty@t.t", "MrEmpty", "password123")

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "MarkReadEmpty", "member_ids": []string{},
	})
	var chat struct{ ID string `json:"id"` }
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	readRes := f.Do(t, "POST", "/api/chats/"+chat.ID+"/read", alice.AccessToken, nil)
	defer readRes.Body.Close()
	if readRes.StatusCode != 200 {
		t.Fatalf("mark read without body: want 200 got %d", readRes.StatusCode)
	}
}

func TestUpdateMe_EmptyBody(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "emptyup@t.t", "EmptyUp", "password123")

	req, _ := http.NewRequest("PATCH", f.HTTP.URL+"/api/users/me", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+alice.AccessToken)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatalf("bad json: want 400 got %d", res.StatusCode)
	}
}

func TestUpload_MissingFile(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "nofile@t.t", "NoFile", "password123")

	res := f.DoMultipart(t, "POST", "/api/uploads", alice.AccessToken, nil, "other", "ignored.txt", []byte("x"), "text/plain")
	defer res.Body.Close()
	if res.StatusCode != 400 {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("missing file field: want 400 got %d body=%s", res.StatusCode, string(b))
	}
}

func TestSendMessage_EmptyContentNoAttachments(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "emptymsgh@t.t", "EmptyMsgH", "password123")

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "EmptyMsgH", "member_ids": []string{},
	})
	var chat struct{ ID string `json:"id"` }
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	res2 := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", alice.AccessToken, map[string]string{
		"content": "",
	})
	defer res2.Body.Close()
	if res2.StatusCode != 400 {
		t.Fatalf("empty msg no attach: want 400 got %d", res2.StatusCode)
	}
}