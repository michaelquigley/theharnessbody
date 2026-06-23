package command

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestDispatchEmptyReturnsHelp(t *testing.T) {
	r := New()
	if got := r.Dispatch(context.Background(), "   "); !strings.Contains(got, "available commands") {
		t.Fatalf("expected help, got %q", got)
	}
}

func TestDispatchHelpListsCommands(t *testing.T) {
	r := New()
	r.Register("review", "review a target", func(ctx context.Context, args []string) (string, error) { return "", nil })
	got := r.Dispatch(context.Background(), "help")
	if !strings.Contains(got, "available commands") || !strings.Contains(got, "review") || !strings.Contains(got, "review a target") {
		t.Fatalf("expected help listing review, got %q", got)
	}
}

func TestDispatchUnknownAppendsHelp(t *testing.T) {
	r := New()
	got := r.Dispatch(context.Background(), "foo bar")
	if !strings.Contains(got, "unknown command 'foo'") {
		t.Fatalf("expected unknown command, got %q", got)
	}
	if !strings.Contains(got, "available commands") {
		t.Fatalf("expected help appended, got %q", got)
	}
}

func TestDispatchRoutesArgs(t *testing.T) {
	r := New()
	var gotArgs []string
	r.Register("review", "", func(ctx context.Context, args []string) (string, error) {
		gotArgs = args
		return "reviewed " + strings.Join(args, ","), nil
	})
	got := r.Dispatch(context.Background(), "review a b")
	if len(gotArgs) != 2 || gotArgs[0] != "a" || gotArgs[1] != "b" {
		t.Fatalf("args not routed: %v", gotArgs)
	}
	if got != "reviewed a,b" {
		t.Fatalf("unexpected reply %q", got)
	}
}

func TestDispatchRendersError(t *testing.T) {
	r := New()
	r.Register("review", "", func(ctx context.Context, args []string) (string, error) {
		return "", errors.New("boom")
	})
	if got := r.Dispatch(context.Background(), "review"); !strings.Contains(got, "error: boom") {
		t.Fatalf("expected rendered error, got %q", got)
	}
}

func TestDispatchCaseInsensitive(t *testing.T) {
	r := New()
	called := false
	r.Register("Review", "", func(ctx context.Context, args []string) (string, error) {
		called = true
		return "ok", nil
	})
	got := r.Dispatch(context.Background(), "REVIEW")
	if !called || strings.Contains(got, "unknown") {
		t.Fatalf("expected case-insensitive match, got %q (called=%v)", got, called)
	}
}

func TestRegisterRejectsBadRegistrations(t *testing.T) {
	ok := func(ctx context.Context, args []string) (string, error) { return "ok", nil }
	cases := map[string]func(){
		"blank name":      func() { New().Register("", "s", ok) },
		"whitespace name": func() { New().Register("   ", "s", ok) },
		"reserved help":   func() { New().Register("Help", "s", ok) },
		"nil handler":     func() { New().Register("x", "s", nil) },
	}
	for name, fn := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("expected Register to panic for %s", name)
				}
			}()
			fn()
		})
	}
}

func TestDispatchPropagatesContext(t *testing.T) {
	r := New()
	type ctxKey string
	r.Register("ping", "", func(ctx context.Context, args []string) (string, error) {
		if ctx.Value(ctxKey("k")) != "v" {
			t.Error("context not propagated to handler")
		}
		return "pong", nil
	})
	ctx := context.WithValue(context.Background(), ctxKey("k"), "v")
	if got := r.Dispatch(ctx, "ping"); got != "pong" {
		t.Fatalf("unexpected reply %q", got)
	}
}
