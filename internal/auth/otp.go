package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/untittledgoosegame/core-backend/pkg/apperr"
)

const (
	codeTTL        = 10 * time.Minute
	codeResendWait = time.Minute
	maxCodeTries   = 5
)

var (
	errCodeExpired = apperr.Unauthorized("code expired or too many attempts, request a new one")
	errCodeInvalid = apperr.Unauthorized("invalid code")
	errCodeTooSoon = apperr.TooMany("wait a minute before requesting another code")
)

// verifyScript checks a code and counts the attempt in one atomic step, so a code
// can be used only once and the attempt limit holds under concurrent requests.
// Returns 1 = ok, 0 = wrong code, -1 = no code / locked out.
var verifyScript = redis.NewScript(`
local stored = redis.call('GET', KEYS[1])
if not stored then return -1 end
if stored == ARGV[1] then
	redis.call('DEL', KEYS[1], KEYS[2])
	return 1
end
local tries = redis.call('INCR', KEYS[2])
redis.call('PEXPIRE', KEYS[2], redis.call('PTTL', KEYS[1]))
if tries >= tonumber(ARGV[2]) then
	redis.call('DEL', KEYS[1], KEYS[2])
	return -1
end
return 0
`)

type otpStore struct {
	rdb    *redis.Client
	secret []byte
}

func newOTPStore(rdb *redis.Client, secret string) *otpStore {
	return &otpStore{rdb: rdb, secret: []byte(secret)}
}

func otpKey(id string) string      { return "otp:" + id }
func otpTriesKey(id string) string { return "otp:tries:" + id }
func otpCoolKey(id string) string  { return "otp:cooldown:" + id }

// Only an HMAC of the code is stored, keyed by the server secret.
func (s *otpStore) hash(id, code string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(id + ":" + code))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *otpStore) issue(ctx context.Context, id string) (string, error) {
	fresh, err := s.rdb.SetNX(ctx, otpCoolKey(id), 1, codeResendWait).Result()
	if err != nil {
		return "", err
	}
	if !fresh {
		return "", errCodeTooSoon
	}

	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	code := fmt.Sprintf("%06d", n.Int64())
	_, err = s.rdb.TxPipelined(ctx, func(p redis.Pipeliner) error {
		p.Set(ctx, otpKey(id), s.hash(id, code), codeTTL)
		p.Del(ctx, otpTriesKey(id))
		return nil
	})
	return code, err
}

func (s *otpStore) verify(ctx context.Context, id, code string) error {
	res, err := verifyScript.Run(ctx, s.rdb,
		[]string{otpKey(id), otpTriesKey(id)}, s.hash(id, code), maxCodeTries).Int()
	if err != nil {
		return err
	}
	switch res {
	case 1:
		return nil
	case 0:
		return errCodeInvalid
	}
	return errCodeExpired
}

// discard drops a code that was never delivered so the user can retry immediately.
func (s *otpStore) discard(ctx context.Context, id string) error {
	return s.rdb.Del(ctx, otpKey(id), otpTriesKey(id), otpCoolKey(id)).Err()
}
