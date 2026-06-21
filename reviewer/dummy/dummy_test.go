package dummy

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/michaelquigley/theharnessbody/reviewer"
)

func TestDummyReturnsConfiguredRawAndCapturesRequests(t *testing.T) {
	want := json.RawMessage(`{"summary":"s","findings":[]}`)
	r := New(Options{Raw: want})

	resp, err := r.Review(context.Background(), reviewer.ReviewRequest{Prompt: "p", Schema: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	if string(resp.Raw) != string(want) {
		t.Fatalf("raw = %s, want %s", resp.Raw, want)
	}

	reqs := r.Requests()
	if len(reqs) != 1 || reqs[0].Prompt != "p" {
		t.Fatalf("requests not captured: %+v", reqs)
	}
}
