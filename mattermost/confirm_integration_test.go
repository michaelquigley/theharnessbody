//go:build integration

// Live smoke tests against a real Mattermost server. Gated on env:
//
//	THB_MM_URL      server base URL (e.g. https://mm.example.com)
//	THB_MM_TOKEN    bot token, or
//	THB_MM_TOKEN_ENV name of the env var holding the token
//	THB_MM_CHANNEL  channel id (for the post test)
//
// Run with: go test -tags integration -v ./mattermost/
// Each test skips when its required env is unset.
package mattermost

import (
	"context"
	"os"
	"testing"
)

func TestConfirmPostMessage(t *testing.T) {
	url := os.Getenv("THB_MM_URL")
	channel := os.Getenv("THB_MM_CHANNEL")
	if url == "" || channel == "" {
		t.Skip("set THB_MM_URL and THB_MM_CHANNEL to run the live post smoke test")
	}
	c := NewClient(Config{
		URL:      url,
		Token:    os.Getenv("THB_MM_TOKEN"),
		TokenEnv: os.Getenv("THB_MM_TOKEN_ENV"),
	})
	if err := c.PostMessage(channel, "theharnessbody mattermost smoke test"); err != nil {
		t.Fatalf("PostMessage failed: %v", err)
	}
}

func TestConfirmConnect(t *testing.T) {
	url := os.Getenv("THB_MM_URL")
	if url == "" {
		t.Skip("set THB_MM_URL to run the live connect smoke test")
	}
	c := NewClient(Config{
		URL:      url,
		Token:    os.Getenv("THB_MM_TOKEN"),
		TokenEnv: os.Getenv("THB_MM_TOKEN_ENV"),
	})
	if err := c.Start(func(ctx context.Context, command string) string { return "" }); err != nil {
		t.Fatalf("Start failed (auth + identity + websocket): %v", err)
	}
	c.Stop()
}
