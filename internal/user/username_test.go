package user

import (
	"testing"
	"time"
)

func TestUsernameCandidates(t *testing.T) {
	got := usernameCandidates("  Zoë  Ann-Marie ")
	want := []string{"zoeannmarie", "zoe.ann.marie", "zoe_ann_marie", "zoem", "zmarie", "marie.zoe", "zoe"}
	for i, w := range want {
		if got[i] != w {
			t.Fatalf("candidate %d = %q, want %q (all: %v)", i, got[i], w, got)
		}
	}
	for _, c := range got {
		if p := UsernameProblem(c); p != "" {
			t.Errorf("invalid candidate %q: %s", c, p)
		}
	}
	if len(usernameCandidates("राहुल")) < suggestCount {
		t.Error("non-latin name should still yield candidates")
	}
}

func TestUsernameProblem(t *testing.T) {
	for name, ok := range map[string]bool{
		"goose_fan": true, "a.b": true, "ab": false, "_abc": false, "abc.": false,
		"a..b": false, "Abc": false, "admin": false, "abc-d": false,
	} {
		if (UsernameProblem(name) == "") != ok {
			t.Errorf("%q valid=%v, want %v", name, !ok, ok)
		}
	}
}

func TestNormalizeProfile(t *testing.T) {
	now := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	base := Profile{Name: " Goose ", Username: "Goose.Fan", DOB: "2000-01-31"}

	p, err := NormalizeProfile(base, now)
	if err != nil || p.Username != "goose.fan" || p.Name != "Goose" {
		t.Fatalf("got %+v, %v", p, err)
	}
	for _, bad := range []Profile{
		{Name: "", Username: "goose", DOB: "2000-01-01"},
		{Name: "G", Username: "goose", DOB: "2014-01-01"},
		{Name: "G", Username: "goose", DOB: "01/01/2000"},
		{Name: "G", Username: "goose", DOB: "2000-01-01", AvatarURL: "http://x.com/a.png"},
	} {
		if _, err := NormalizeProfile(bad, now); err == nil {
			t.Errorf("expected error for %+v", bad)
		}
	}
}
