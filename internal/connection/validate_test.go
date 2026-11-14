package connection

import (
	"testing"
	"time"
)

func TestEventValidate(t *testing.T) {
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	valid := Event{
		ID: "evt_1", Type: EventConnected, OccurredAt: now, UserID: "6af763b09c606f1eb93dcedc",
		Provider: Spotify, Account: &Account{ID: "sp_1", AvatarURL: "https://i.scdn.co/a.png"},
	}
	if err := valid.Validate(now); err != nil {
		t.Fatalf("valid event rejected: %v", err)
	}
	disconnected := valid
	disconnected.Type, disconnected.Account = EventDisconnected, nil
	if err := disconnected.Validate(now); err != nil {
		t.Fatalf("disconnect without account rejected: %v", err)
	}

	for name, mutate := range map[string]func(*Event){
		"no id":            func(e *Event) { e.ID = "" },
		"bad type":         func(e *Event) { e.Type = "connection.deleted" },
		"future":           func(e *Event) { e.OccurredAt = now.Add(time.Hour) },
		"unknown provider": func(e *Event) { e.Provider = "myspace" },
		"no account":       func(e *Event) { e.Account = nil },
		"http avatar":      func(e *Event) { e.Account = &Account{ID: "x", AvatarURL: "http://x.com/a.png"} },
	} {
		e := valid
		mutate(&e)
		if err := e.Validate(now); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}
