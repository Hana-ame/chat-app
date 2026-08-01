package service_test

import (
	"sync"
	"testing"

	"github.com/Hana-ame/chat-app/server/internal/testutil"
)

func TestChatService_CreateOrGetNotificationsChat_RepeatedCalls(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "notify_rep@x.com", "NotifyRep")

	first, err := f.Server.Services.Chat.CreateOrGetNotificationsChat(f.Ctx(), a)
	testutil.RequireNoError(t, err)
	second, err := f.Server.Services.Chat.CreateOrGetNotificationsChat(f.Ctx(), a)
	testutil.RequireNoError(t, err)
	testutil.RequireEqual(t, first.ID, second.ID)
}

func TestChatService_CreateOrGetNotificationsChat_Concurrent(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "notify_conc@x.com", "NotifyConc")

	const n = 12
	ids := make([]string, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			chat, err := f.Server.Services.Chat.CreateOrGetNotificationsChat(f.Ctx(), a)
			if err == nil {
				ids[i] = chat.ID
			} else {
				t.Errorf("concurrent CreateOrGetNotificationsChat: %v", err)
			}
		}(i)
	}
	wg.Wait()

	testutil.RequireEqual(t, ids[0], ids[n-1])
	for i := 1; i < n-1; i++ {
		if ids[i] != ids[0] {
			t.Fatalf("expected all goroutines to get the same notify chat, got %v", ids)
		}
	}
}

func TestChatService_CreateOrGetNotificationsChat_PerUser(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "notify_a@x.com", "NotifyA")
	b := createTestUser(t, f, "notify_b@x.com", "NotifyB")

	ca, err := f.Server.Services.Chat.CreateOrGetNotificationsChat(f.Ctx(), a)
	testutil.RequireNoError(t, err)
	cb, err := f.Server.Services.Chat.CreateOrGetNotificationsChat(f.Ctx(), b)
	testutil.RequireNoError(t, err)
	testutil.RequireEqual(t, ca.ID == cb.ID, false)
}
