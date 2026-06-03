package ratelimit

import (
	"sort"
	"strings"
	"testing"
)

// FuzzBuildKey verifies invariants of buildKey that must hold for any input:
//
//  1. Deterministic: same components map produces the same key.
//  2. Order-independent: keys built from differently-iterated maps are equal
//     (this is implicit from #1 because map iteration in Go is randomised,
//     and buildKey sorts internally; #1 with multiple calls covers it).
//  3. Key/value separator integrity: when the separator does not appear in
//     any key or value, the number of "<sep>" occurrences equals N-1 for
//     N components, and the number of "=" equals N.
//  4. Composition order: components appear sorted by key name.
//
// To run with go fuzz (Go 1.18+):
//
//	go test -fuzz=FuzzBuildKey -fuzztime=30s ./pkg/ratelimit/
func FuzzBuildKey(f *testing.F) {
	// Seed corpus — sensible inputs to start fuzzing from.
	f.Add("user_id", "alice", "path", "/api")
	f.Add("a", "b", "c", "d")
	f.Add("", "", "k", "v")       // empty key+value
	f.Add("k1", "v1", "k1", "v2") // duplicate keys
	f.Add("=", "=", "==", "==")   // edge: '=' in keys/values

	f.Fuzz(func(t *testing.T, k1, v1, k2, v2 string) {
		m := &RateLimitManager{}
		comps := map[string]string{k1: v1, k2: v2}
		sep := "|"

		// Invariant 1: deterministic.
		key1 := m.buildKey(comps, sep)
		key2 := m.buildKey(comps, sep)
		if key1 != key2 {
			t.Errorf("buildKey not deterministic:\n  call1=%q\n  call2=%q", key1, key2)
		}

		// Invariant 3 + 4: only checked when separator is absent from inputs
		// (otherwise = and | counts cannot be predicted).
		safe := !strings.ContainsAny(k1, "=|") && !strings.ContainsAny(v1, "=|") &&
			!strings.ContainsAny(k2, "=|") && !strings.ContainsAny(v2, "=|")

		if safe {
			n := len(comps) // 1 or 2 depending on whether k1 == k2
			gotEq := strings.Count(key1, "=")
			if gotEq != n {
				t.Errorf("expected %d '=' but got %d in %q (inputs k1=%q v1=%q k2=%q v2=%q)",
					n, gotEq, key1, k1, v1, k2, v2)
			}
			if n > 1 {
				gotSep := strings.Count(key1, sep)
				if gotSep != n-1 {
					t.Errorf("expected %d sep but got %d in %q", n-1, gotSep, key1)
				}
				// Invariant 4: keys are in sorted order.
				keys := make([]string, 0, n)
				for k := range comps {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				wantPrefix := keys[0] + "="
				if !strings.HasPrefix(key1, wantPrefix) {
					t.Errorf("expected key to start with %q but got %q", wantPrefix, key1)
				}
			}
		}
	})
}

// TestBuildKey_KnownVectors checks specific expected outputs.
// This complements the fuzz test by anchoring the contract to concrete values.
func TestBuildKey_KnownVectors(t *testing.T) {
	m := &RateLimitManager{}

	cases := []struct {
		name string
		in   map[string]string
		sep  string
		want string
	}{
		{
			name: "single component",
			in:   map[string]string{"user_id": "alice"},
			sep:  "|",
			want: "user_id=alice",
		},
		{
			name: "two components sorted alphabetically",
			in:   map[string]string{"user_id": "alice", "path": "/api"},
			sep:  "|",
			want: "path=/api|user_id=alice",
		},
		{
			name: "empty components",
			in:   map[string]string{},
			sep:  "|",
			want: "",
		},
		{
			name: "nil components",
			in:   nil,
			sep:  "|",
			want: "",
		},
		{
			name: "custom separator",
			in:   map[string]string{"a": "1", "b": "2"},
			sep:  "::",
			want: "a=1::b=2",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := m.buildKey(tc.in, tc.sep)
			if got != tc.want {
				t.Errorf("buildKey() = %q, want %q", got, tc.want)
			}
		})
	}
}
