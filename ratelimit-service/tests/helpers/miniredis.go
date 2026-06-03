// Package helpers contains reusable test utilities for the ratelimit-service test suite.
//
// This file: miniredis-based environment builder.
package helpers

import (
	"testing"

	"ratelimit-service/pkg/ratelimit"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
)

// Env bundles the components most tests need: an in-process Redis (miniredis),
// a wired-up RedisClient, and a RateLimitManager. Use NewEnv to build one.
//
// The fields are exported so tests can poke at internals when necessary;
// most tests will only touch Manager and Redis.
type Env struct {
	Miniredis *miniredis.Miniredis
	Redis     *ratelimit.RedisClient
	Manager   *ratelimit.RateLimitManager
}

// NewEnv constructs a fresh test environment with miniredis as the Redis backend.
// Cleanup is registered with t.Cleanup, so callers don't need to defer anything.
func NewEnv(t *testing.T) *Env {
	t.Helper()

	mr := miniredis.RunT(t)

	rc, err := ratelimit.NewRedisClient(mr.Addr(), "", 0)
	require.NoError(t, err, "creating RedisClient against miniredis")

	mgr := ratelimit.NewRateLimitManager(rc)
	rc.SetManager(mgr)

	t.Cleanup(func() {
		_ = rc.Close()
		mr.Close()
	})

	return &Env{
		Miniredis: mr,
		Redis:     rc,
		Manager:   mgr,
	}
}
