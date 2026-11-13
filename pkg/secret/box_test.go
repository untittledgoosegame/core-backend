package secret

import (
	"bytes"
	"testing"
)

func TestBox(t *testing.T) {
	box, err := New(bytes.Repeat([]byte{7}, KeySize))
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := box.Seal("JBSWY3DPEHPK3PXP", "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if sealed == "JBSWY3DPEHPK3PXP" {
		t.Fatal("value not encrypted")
	}
	if got, err := box.Open(sealed, "user-1"); err != nil || got != "JBSWY3DPEHPK3PXP" {
		t.Fatalf("open = %q, %v", got, err)
	}
	if _, err := box.Open(sealed, "user-2"); err == nil {
		t.Fatal("opened with the wrong context")
	}
	if _, err := New([]byte("short")); err == nil {
		t.Fatal("accepted a short key")
	}
}
