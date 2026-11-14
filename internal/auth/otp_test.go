package auth

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/redis/go-redis/v9"
)

func testRedis(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("REDIS_URL")
	if addr == "" {
		addr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr, DB: 15})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis not available at %s: %v", addr, err)
	}
	t.Cleanup(func() { rdb.FlushDB(context.Background()); rdb.Close() })
	return rdb
}

func TestOTPStore(t *testing.T) {
	ctx := context.Background()
	s := newOTPStore(testRedis(t), "test-secret")

	code, err := s.issue(ctx, "login:a@b.c")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.issue(ctx, "login:a@b.c"); !errors.Is(err, errCodeTooSoon) {
		t.Fatalf("resend within cooldown: got %v", err)
	}
	if err := s.verify(ctx, "login:a@b.c", "wrong!"); !errors.Is(err, errCodeInvalid) {
		t.Fatalf("wrong code: got %v", err)
	}
	if err := s.verify(ctx, "login:a@b.c", code); err != nil {
		t.Fatalf("right code: got %v", err)
	}
	if err := s.verify(ctx, "login:a@b.c", code); !errors.Is(err, errCodeExpired) {
		t.Fatalf("reused code: got %v", err)
	}

	code, _ = s.issue(ctx, "login:x@y.z")
	for range maxCodeTries - 1 {
		_ = s.verify(ctx, "login:x@y.z", "wrong!")
	}
	if err := s.verify(ctx, "login:x@y.z", "wrong!"); !errors.Is(err, errCodeExpired) {
		t.Fatalf("final wrong attempt should lock out: got %v", err)
	}
	if err := s.verify(ctx, "login:x@y.z", code); !errors.Is(err, errCodeExpired) {
		t.Fatalf("right code after lockout: got %v", err)
	}

	_ = s.discard(ctx, "login:x@y.z")
	if _, err := s.issue(ctx, "login:x@y.z"); err != nil {
		t.Fatalf("issue after discard: got %v", err)
	}
}
