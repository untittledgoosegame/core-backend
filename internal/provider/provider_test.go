package provider

import "testing"

func TestFlags(t *testing.T) {
	flags := Flags(map[Provider]bool{Spotify: true, "myspace": true})
	if len(flags) != len(All) {
		t.Fatalf("got %d flags, want %d", len(flags), len(All))
	}
	if !flags[Spotify] || flags[YouTube] {
		t.Fatalf("flags = %v", flags)
	}
	if _, ok := flags["myspace"]; ok {
		t.Fatal("unknown provider leaked into flags")
	}
}
