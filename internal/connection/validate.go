package connection

import (
	"net/url"
	"time"

	"github.com/untittledgoosegame/core-backend/pkg/apperr"
)

const (
	maxClockSkew  = 5 * time.Minute
	maxEventIDLen = 128
)

func (e Event) Validate(now time.Time) error {
	switch {
	case e.ID == "" || len(e.ID) > maxEventIDLen:
		return apperr.BadRequest("id is required (max 128 characters)")
	case e.Type != EventConnected && e.Type != EventUpdated && e.Type != EventDisconnected:
		return apperr.BadRequest("type must be connection.connected, connection.updated or connection.disconnected")
	case e.OccurredAt.IsZero():
		return apperr.BadRequest("occurred_at is required")
	case e.OccurredAt.After(now.Add(maxClockSkew)):
		return apperr.BadRequest("occurred_at is in the future")
	case e.UserID == "":
		return apperr.BadRequest("user_id is required")
	case !e.Provider.Valid():
		return apperr.BadRequest("unknown provider " + string(e.Provider))
	}
	if e.Type == EventDisconnected {
		return nil
	}
	if e.Account == nil || e.Account.ID == "" {
		return apperr.BadRequest("account.id is required for connected and updated events")
	}
	for field, raw := range map[string]string{"account.avatar_url": e.Account.AvatarURL, "account.profile_url": e.Account.ProfileURL} {
		if raw == "" {
			continue
		}
		if u, err := url.Parse(raw); err != nil || u.Scheme != "https" || u.Host == "" {
			return apperr.BadRequest(field + " must be an https URL")
		}
	}
	return nil
}
