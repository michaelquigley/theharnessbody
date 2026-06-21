// Package dummy is a reviewer that returns a fixed response without invoking any
// backend. It exists so the reviewer plumbing — request capture, response
// passthrough, and the caller's validation step — can be exercised in tests and
// scaffolding without a model.
package dummy

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/michaelquigley/theharnessbody/reviewer"
)

// Options configures a dummy reviewer.
type Options struct {
	Raw        json.RawMessage
	Err        error
	UsageNotes string
}

// Reviewer returns a fixed response and records the requests it received.
type Reviewer struct {
	mu       sync.Mutex
	raw      json.RawMessage
	err      error
	usage    string
	requests []reviewer.ReviewRequest
}

// New returns a dummy reviewer. With no options it returns an empty JSON object;
// supply Options.Raw with a schema-valid payload when a test needs one.
func New(options ...Options) *Reviewer {
	option := Options{
		Raw:        defaultRaw(),
		UsageNotes: "dummy reviewer",
	}
	if len(options) > 0 {
		option = options[0]
		if len(option.Raw) == 0 {
			option.Raw = defaultRaw()
		}
		if option.UsageNotes == "" {
			option.UsageNotes = "dummy reviewer"
		}
	}
	return &Reviewer{
		raw:   append(json.RawMessage(nil), option.Raw...),
		err:   option.Err,
		usage: option.UsageNotes,
	}
}

func (r *Reviewer) Review(ctx context.Context, req reviewer.ReviewRequest) (reviewer.ReviewResponse, error) {
	if err := ctx.Err(); err != nil {
		return reviewer.ReviewResponse{}, err
	}

	r.mu.Lock()
	r.requests = append(r.requests, cloneRequest(req))
	raw := append(json.RawMessage(nil), r.raw...)
	err := r.err
	usage := r.usage
	r.mu.Unlock()

	if err != nil {
		return reviewer.ReviewResponse{}, err
	}

	return reviewer.ReviewResponse{
		Raw:        raw,
		UsageNotes: usage,
	}, nil
}

// Requests returns the requests captured by this reviewer.
func (r *Reviewer) Requests() []reviewer.ReviewRequest {
	r.mu.Lock()
	defer r.mu.Unlock()

	requests := make([]reviewer.ReviewRequest, 0, len(r.requests))
	for _, req := range r.requests {
		requests = append(requests, cloneRequest(req))
	}
	return requests
}

func defaultRaw() json.RawMessage {
	return json.RawMessage("{}")
}

func cloneRequest(req reviewer.ReviewRequest) reviewer.ReviewRequest {
	req.Schema = append(json.RawMessage(nil), req.Schema...)
	return req
}
