package auth

import (
	"strings"
	"testing"
)

func TestAndroidOrigins(t *testing.T) {
	fp := strings.TrimSuffix(strings.Repeat("AB:", 32), ":")
	got, err := AndroidOrigins([]string{fp})
	if err != nil {
		t.Fatal(err)
	}
	want := "android:apk-key-hash:q6urq6urq6urq6urq6urq6urq6urq6urq6urq6urq6s"
	if len(got) != 1 || got[0] != want {
		t.Fatalf("got %v, want %s", got, want)
	}
	if _, err := AndroidOrigins([]string{"AB:CD"}); err == nil {
		t.Fatal("accepted a short fingerprint")
	}
}
