package webhook

import (
	"errors"
	"testing"
	"time"
)

func TestVerify(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	body := []byte(`{"id":"evt_1"}`)
	header := Sign("s3cret", body, now)

	if err := Verify("s3cret", header, body, now.Add(time.Minute), 5*time.Minute); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
	for name, tc := range map[string]struct {
		secret, header string
		body           []byte
		at             time.Time
		want           error
	}{
		"tampered body": {"s3cret", header, []byte(`{"id":"evt_2"}`), now, ErrBadSignature},
		"wrong secret":  {"other", header, body, now, ErrBadSignature},
		"too old":       {"s3cret", header, body, now.Add(6 * time.Minute), ErrStaleSignature},
		"no header":     {"s3cret", "", body, now, ErrMissingSignature},
		"no v1":         {"s3cret", "t=1800000000", body, now, ErrMissingSignature},
	} {
		if err := Verify(tc.secret, tc.header, tc.body, tc.at, 5*time.Minute); !errors.Is(err, tc.want) {
			t.Errorf("%s: got %v, want %v", name, err, tc.want)
		}
	}
}
