package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
)

const SignatureHeader = "X-Webhook-Signature"

var (
	ErrMissingSignature = errors.New("missing or malformed signature header")
	ErrBadSignature     = errors.New("signature does not match")
	ErrStaleSignature   = errors.New("signature timestamp outside tolerance")
)

// Sign returns the header value "t=<unix>,v1=<hex hmac-sha256 of "<unix>.<body>">".
func Sign(secret string, body []byte, at time.Time) string {
	ts := strconv.FormatInt(at.Unix(), 10)
	return "t=" + ts + ",v1=" + mac(secret, ts, body)
}

func Verify(secret, header string, body []byte, now time.Time, tolerance time.Duration) error {
	var ts, sig string
	for part := range strings.SplitSeq(header, ",") {
		key, value, _ := strings.Cut(strings.TrimSpace(part), "=")
		switch key {
		case "t":
			ts = value
		case "v1":
			sig = value
		}
	}
	unix, err := strconv.ParseInt(ts, 10, 64)
	if err != nil || sig == "" {
		return ErrMissingSignature
	}
	if age := now.Sub(time.Unix(unix, 0)); age > tolerance || age < -tolerance {
		return ErrStaleSignature
	}
	if !hmac.Equal([]byte(sig), []byte(mac(secret, ts, body))) {
		return ErrBadSignature
	}
	return nil
}

func mac(secret, ts string, body []byte) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(ts))
	h.Write([]byte("."))
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}
