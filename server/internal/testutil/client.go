package testutil

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type Session struct {
	UserID       string `json:"-"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	User         struct {
		ID          string `json:"id"`
		Email       string `json:"email"`
		Username    string `json:"username"`
		AvatarColor string `json:"avatar_color"`
		Status      string `json:"status"`
	} `json:"user"`
}

func (f *Fixture) Register(t *testing.T, email, username, password string) *Session {
	t.Helper()
	body := map[string]string{"email": email, "username": username, "password": password}
	res := f.Do(t, "POST", "/api/auth/register", "", body)
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("register failed: %d %s", res.StatusCode, string(b))
	}
	var s Session
	if err := json.NewDecoder(res.Body).Decode(&s); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	s.UserID = s.User.ID
	return &s
}

func (f *Fixture) Login(t *testing.T, email, password string) *Session {
	t.Helper()
	body := map[string]string{"email": email, "password": password}
	res := f.Do(t, "POST", "/api/auth/login", "", body)
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("login failed: %d %s", res.StatusCode, string(b))
	}
	var s Session
	if err := json.NewDecoder(res.Body).Decode(&s); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	s.UserID = s.User.ID
	return &s
}

func (f *Fixture) Do(t *testing.T, method, path, token string, body interface{}) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, f.HTTP.URL+path, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("http do: %v", err)
	}
	return res
}

func (f *Fixture) DoJSON(t *testing.T, method, path, token string, body, out interface{}) int {
	t.Helper()
	res := f.Do(t, method, path, token, body)
	defer res.Body.Close()
	if out != nil && res.StatusCode >= 200 && res.StatusCode < 300 {
		if err := json.NewDecoder(res.Body).Decode(out); err != nil && err != io.EOF {
			t.Fatalf("decode response: %v", err)
		}
	}
	return res.StatusCode
}

func (f *Fixture) DoMultipart(t *testing.T, method, path, token string, fields map[string]string, fileField, filename string, content []byte) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	mw := newMultipart(&buf)
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	w, err := mw.CreateFormFile(fileField, filename)
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	_, _ = w.Write(content)
	_ = mw.Close()

	req, err := http.NewRequest(method, f.HTTP.URL+path, &buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("http do: %v", err)
	}
	return res
}

func (f *Fixture) WSURL(token string) string {
	u, _ := url.Parse(f.HTTP.URL)
	scheme := "ws"
	if u.Scheme == "https" {
		scheme = "wss"
	}
	return scheme + "://" + u.Host + "/ws?access_token=" + url.QueryEscape(token)
}

// NewRecorder gives a quick httptest recorder for in-process testing.
func NewRecorder() *httptest.ResponseRecorder { return httptest.NewRecorder() }

func ContainsJSONError(body []byte, errCode string) bool {
	return strings.Contains(string(body), `"error":"`+errCode+`"`)
}
