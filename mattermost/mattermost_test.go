package mattermost

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func stubResponder(reply string, captured *string) Responder {
	return func(ctx context.Context, command string) string {
		if captured != nil {
			*captured = command
		}
		return reply
	}
}

func postedEvent(userID, channelID, message string) []byte {
	postJSON, _ := json.Marshal(map[string]string{"user_id": userID, "channel_id": channelID, "message": message})
	dataJSON, _ := json.Marshal(map[string]string{"post": string(postJSON)})
	eventJSON, _ := json.Marshal(map[string]any{"event": "posted", "data": json.RawMessage(dataJSON)})
	return eventJSON
}

func TestNewClientTokenFromEnv(t *testing.T) {
	orig := lookupEnv
	defer func() { lookupEnv = orig }()
	lookupEnv = func(key string) string {
		if key == "MM_TOKEN" {
			return "env-token"
		}
		return ""
	}
	c := NewClient(Config{URL: "http://localhost", TokenEnv: "MM_TOKEN", Token: "fallback-token"})
	if c.token != "env-token" {
		t.Errorf("expected env token, got %q", c.token)
	}
}

func TestNewClientTokenFallback(t *testing.T) {
	orig := lookupEnv
	defer func() { lookupEnv = orig }()
	lookupEnv = func(string) string { return "" }
	c := NewClient(Config{URL: "http://localhost", TokenEnv: "MM_TOKEN", Token: "direct-token"})
	if c.token != "direct-token" {
		t.Errorf("expected direct token, got %q", c.token)
	}
}

// Divergence from sexton: no default trigger word. Empty TriggerWords means
// mention-only operation, which the body can't presume a tool name for.
func TestNewClientNoDefaultTriggerWords(t *testing.T) {
	orig := lookupEnv
	defer func() { lookupEnv = orig }()
	lookupEnv = func(string) string { return "" }
	c := NewClient(Config{URL: "http://localhost", Token: "t"})
	if len(c.cfg.TriggerWords) != 0 {
		t.Errorf("expected no default trigger words, got %v", c.cfg.TriggerWords)
	}
}

func TestStartMissingToken(t *testing.T) {
	orig := lookupEnv
	defer func() { lookupEnv = orig }()
	lookupEnv = func(string) string { return "" }
	c := NewClient(Config{URL: "http://localhost"})
	err := c.Start(stubResponder("", nil))
	if err == nil || !strings.Contains(err.Error(), "token is required") {
		t.Fatalf("expected missing-token error, got %v", err)
	}
}

func TestPostMessage(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/posts" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := &Client{cfg: Config{URL: srv.URL}, token: "test-token", httpClient: srv.Client()}
	if err := c.PostMessage("chan123", "hello"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["channel_id"] != "chan123" || gotBody["message"] != "hello" {
		t.Errorf("unexpected body: %v", gotBody)
	}
}

func TestPostMessagePreservesBasePath(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := &Client{cfg: Config{URL: srv.URL + "/mattermost/"}, token: "test-token", httpClient: srv.Client()}
	if err := c.PostMessage("chan123", "hello"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/mattermost/api/v4/posts" {
		t.Fatalf("request path = %q, want %q", gotPath, "/mattermost/api/v4/posts")
	}
}

func TestSelfMessageSuppression(t *testing.T) {
	called := false
	c := &Client{
		cfg:       Config{URL: "http://localhost"},
		botUserID: "bot123",
		responder: func(ctx context.Context, cmd string) string { called = true; return "x" },
	}
	c.handleMessage(postedEvent("bot123", "chan", "sexton status"))
	if called {
		t.Error("expected the bot's own message to be ignored")
	}
}

func TestAllowedUserFiltering(t *testing.T) {
	userSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "user456", "username": "stranger"})
	}))
	defer userSrv.Close()

	called := false
	c := &Client{
		cfg:        Config{URL: userSrv.URL, AllowedUsers: []string{"michael"}, TriggerWords: []string{"sexton"}},
		botUserID:  "bot123",
		httpClient: userSrv.Client(),
		responder:  func(ctx context.Context, cmd string) string { called = true; return "x" },
		userCache:  make(map[string]string),
	}
	c.handleMessage(postedEvent("user456", "chan", "sexton status"))
	if called {
		t.Error("expected command filtered for non-allowed user")
	}
}

func TestAllowedUserEmptyDispatches(t *testing.T) {
	posted := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/v4/posts" {
			posted = true
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := &Client{
		cfg:        Config{URL: srv.URL, TriggerWords: []string{"sexton"}},
		token:      "t",
		botUserID:  "bot123",
		httpClient: srv.Client(),
		responder:  func(ctx context.Context, cmd string) string { return "reply" },
		userCache:  make(map[string]string),
	}
	c.handleMessage(postedEvent("user789", "chan", "sexton status"))
	if !posted {
		t.Error("expected reply posted when allowed users is empty")
	}
}

func TestHandleMessageRoutesStrippedCommand(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	var gotCmd string
	c := &Client{
		cfg:        Config{URL: srv.URL, TriggerWords: []string{"sexton"}},
		token:      "t",
		botUserID:  "bot123",
		httpClient: srv.Client(),
		responder:  func(ctx context.Context, cmd string) string { gotCmd = cmd; return "ok" },
		userCache:  make(map[string]string),
	}
	c.handleMessage(postedEvent("user1", "chan", "sexton sync notes"))
	if gotCmd != "sync notes" {
		t.Errorf("expected stripped command 'sync notes', got %q", gotCmd)
	}
}

func TestEmptyReplyNotPosted(t *testing.T) {
	posted := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posted = true
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := &Client{
		cfg:        Config{URL: srv.URL, TriggerWords: []string{"sexton"}},
		token:      "t",
		botUserID:  "bot123",
		httpClient: srv.Client(),
		responder:  func(ctx context.Context, cmd string) string { return "" },
		userCache:  make(map[string]string),
	}
	c.handleMessage(postedEvent("user1", "chan", "sexton status"))
	if posted {
		t.Error("expected an empty reply not to be posted")
	}
}

func TestUsernameCaching(t *testing.T) {
	apiCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalls++
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "user1", "username": "michael"})
	}))
	defer srv.Close()

	c := &Client{cfg: Config{URL: srv.URL}, httpClient: srv.Client(), userCache: make(map[string]string)}
	for i := 0; i < 2; i++ {
		u, err := c.resolveUsername("user1")
		if err != nil || u != "michael" {
			t.Fatalf("resolveUsername = %q, %v", u, err)
		}
	}
	if apiCalls != 1 {
		t.Errorf("expected 1 api call (cached), got %d", apiCalls)
	}
}

func TestExtractCommand(t *testing.T) {
	c := &Client{cfg: Config{TriggerWords: []string{"sexton"}}, botUserID: "bot123"}
	mentions, _ := json.Marshal([]string{"bot123"})
	multi, _ := json.Marshal([]string{"bot123", "bot456"})

	cases := []struct {
		name     string
		message  string
		mentions string
		want     string
		ok       bool
	}{
		{"mention", "@sexton-laptop status", string(mentions), "status", true},
		{"mention-multi", "@a @b sync grimoire", string(multi), "sync grimoire", true},
		{"trigger", "sexton sync notes", "", "sync notes", true},
		{"no-match", "hello world", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := c.extractCommand(tc.message, tc.mentions)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("extractCommand(%q) = %q,%v want %q,%v", tc.message, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestBuildWebSocketURL(t *testing.T) {
	tests := []struct{ url, expect string }{
		{"https://mm.local", "wss://mm.local/api/v4/websocket"},
		{"https://mm.local/", "wss://mm.local/api/v4/websocket"},
		{"https://mm.local/mattermost", "wss://mm.local/mattermost/api/v4/websocket"},
		{"http://mm.local:8065", "ws://mm.local:8065/api/v4/websocket"},
	}
	for _, tt := range tests {
		c := &Client{cfg: Config{URL: tt.url}}
		got, err := c.buildWebSocketURL()
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", tt.url, err)
		}
		if got != tt.expect {
			t.Errorf("for %q: expected %q, got %q", tt.url, tt.expect, got)
		}
	}
}

func TestStripTriggerWord(t *testing.T) {
	cases := []struct {
		name    string
		text    string
		words   []string
		want    string
		matched bool
	}{
		{"match", "sexton status", []string{"sexton"}, "status", true},
		{"case-insensitive", "SEXTON status", []string{"sexton"}, "status", true},
		{"bare", "sexton", []string{"sexton"}, "", true},
		{"no-match", "other status", []string{"sexton"}, "", false},
		{"partial-no-match", "sextonbot status", []string{"sexton"}, "", false},
		{"multiple-words", "bot sync notes", []string{"sexton", "bot"}, "sync notes", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := StripTriggerWord(tc.text, tc.words)
			if ok != tc.matched || got != tc.want {
				t.Fatalf("StripTriggerWord(%q) = %q,%v want %q,%v", tc.text, got, ok, tc.want, tc.matched)
			}
		})
	}
}

func newTestClient(srv *httptest.Server) *Client {
	return &Client{
		cfg:        Config{URL: srv.URL, TriggerWords: []string{"sexton"}},
		token:      "test-token",
		httpClient: srv.Client(),
		userCache:  make(map[string]string),
		stopCh:     make(chan struct{}),
		doneCh:     make(chan struct{}),
	}
}

func TestStartAuthSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4/users/me" {
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "bot123", "username": "terminus-test"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	// auth should succeed; the WebSocket dial fails (httptest has no WS upgrade),
	// so Start returns a connect error after identity is resolved.
	err := c.Start(stubResponder("", nil))
	if err == nil {
		c.Stop()
	} else if strings.Contains(err.Error(), "authenticate") {
		t.Errorf("auth should have succeeded, got: %v", err)
	}
	if c.botUserID != "bot123" || c.botUsername != "terminus-test" {
		t.Errorf("identity not resolved: %q / %q", c.botUserID, c.botUsername)
	}
}

func TestStartAuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"invalid token"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	err := c.Start(stubResponder("", nil))
	if err == nil {
		c.Stop()
		t.Fatal("expected error for auth failure")
	}
	if !strings.Contains(err.Error(), "authenticate") {
		t.Errorf("expected auth error, got: %v", err)
	}
}

func TestStartNilResponder(t *testing.T) {
	c := NewClient(Config{URL: "http://localhost", Token: "t"})
	if err := c.Start(nil); err == nil || !strings.Contains(err.Error(), "responder is required") {
		t.Fatalf("expected nil-responder error, got %v", err)
	}
}

func TestStartRejectsDoubleStart(t *testing.T) {
	c := NewClient(Config{URL: "http://localhost", Token: "t"})
	c.lifeMu.Lock()
	c.state = stateStarted // simulate an already-running client
	c.lifeMu.Unlock()
	if err := c.Start(stubResponder("", nil)); err == nil || !strings.Contains(err.Error(), "twice") {
		t.Fatalf("expected double-start to be rejected, got %v", err)
	}
}

func TestStopBeforeStartIsNoop(t *testing.T) {
	c := NewClient(Config{URL: "http://localhost", Token: "t"})
	c.Stop() // never started
	c.Stop() // idempotent
}

// Regression: a pre-start Stop must not consume the lifecycle and leave a later
// successfully-started client unstoppable.
func TestStopBeforeStartThenRealStop(t *testing.T) {
	c := NewClient(Config{URL: "http://localhost", Token: "t"})
	c.Stop() // pre-start: a no-op that must NOT consume the lifecycle

	// stand in for the state a successful Start leaves behind, plus a fake listener
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.lifeMu.Lock()
	c.state = stateStarted
	c.lifeMu.Unlock()
	go func() { <-c.stopCh; close(c.doneCh) }()

	done := make(chan struct{})
	go func() {
		c.Stop() // must actually stop
		c.Stop() // and remain idempotent afterward
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop after a real start blocked — pre-start Stop consumed the lifecycle")
	}
	if c.ctx.Err() == nil {
		t.Fatal("Stop did not cancel the context")
	}
}
