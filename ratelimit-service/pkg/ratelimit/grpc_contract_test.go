package ratelimit

import (
	"testing"

	"github.com/stretchr/testify/assert"

	pb "github.com/envoyproxy/go-control-plane/envoy/service/ratelimit/v3"

	// RateLimitDescriptor and its nested Entry type live in the common ratelimit
	// extensions package, which is imported by the service proto. This is
	// consistent with the envoy API definition:
	//   envoy/service/ratelimit/v3/rls.proto imports
	//   envoy/extensions/common/ratelimit/v3/ratelimit.proto
	// See envoy/extensions/common/ratelimit/v3/ratelimit.proto for the type definition.
	rl "github.com/envoyproxy/go-control-plane/envoy/extensions/common/ratelimit/v3"
)

// TestGRPCContract_BuildKeyFromRequest pins the wire-to-key mapping that the
// Envoy → ratelimit-service contract relies on. If the proto types change or
// the join logic in buildKeyFromRequest drifts, this test fails loudly.
//
// The expected strings here are exactly what RateLimitManager.findMatchingRule
// will see, so any drift here is a hard production regression.
func TestGRPCContract_BuildKeyFromRequest(t *testing.T) {
	// build constructs a RateLimitRequest from a list of descriptor entry
	// slices. Each inner slice becomes one RateLimitDescriptor.
	type Entry struct {
		Key   string
		Value string
	}

	build := func(domain string, descriptors ...[]Entry) *pb.RateLimitRequest {
		req := &pb.RateLimitRequest{Domain: domain}
		for _, desc := range descriptors {
			d := &rl.RateLimitDescriptor{}
			for _, e := range desc {
				d.Entries = append(d.Entries, &rl.RateLimitDescriptor_Entry{
					Key:   e.Key,
					Value: e.Value,
				})
			}
			req.Descriptors = append(req.Descriptors, d)
		}
		return req
	}

	cases := []struct {
		name string
		req  *pb.RateLimitRequest
		want string
	}{
		{
			name: "domain only, no descriptors",
			req:  build("auth_limit"),
			want: "domain=auth_limit",
		},
		{
			name: "single descriptor with one entry (path)",
			req:  build("auth_limit", []Entry{{"path", "/api/v1/orders"}}),
			want: "domain=auth_limit|path=/api/v1/orders",
		},
		{
			name: "single descriptor with two entries (path, user_id)",
			req: build("auth_limit", []Entry{
				{"path", "/api/v1/orders"},
				{"user_id", "alice"},
			}),
			want: "domain=auth_limit|path=/api/v1/orders|user_id=alice",
		},
		{
			name: "duplicate keys across descriptors: first one wins",
			req: build("auth_limit",
				[]Entry{{"path", "/first"}},
				[]Entry{{"path", "/second"}}, // duplicate path — should be ignored
			),
			want: "domain=auth_limit|path=/first",
		},
		{
			name: "multiple descriptors, no duplicate keys",
			req: build("auth_limit",
				[]Entry{{"path", "/api"}},
				[]Entry{{"user_id", "alice"}},
			),
			want: "domain=auth_limit|path=/api|user_id=alice",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildKeyFromRequest(tc.req)
			assert.Equal(t, tc.want, got)
		})
	}
}
