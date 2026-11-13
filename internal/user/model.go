package user

import (
	"encoding/base64"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/untittledgoosegame/core-backend/internal/provider"
)

type User struct {
	ID             bson.ObjectID         `bson:"_id"`
	Email          string                `bson:"email,omitempty"`
	Phone          string                `bson:"phone,omitempty"`
	TruecallerID   string                `bson:"truecaller_id,omitempty"`
	WebAuthnUserID []byte                `bson:"webauthn_id"`
	Passkeys       []webauthn.Credential `bson:"passkeys,omitempty"`
	TOTP           TOTP                  `bson:"totp"`
	Profile        Profile               `bson:"profile"`
	// Connections mirrors the connections collection's connected flags so profiles render without a join.
	Connections map[provider.Provider]bool `bson:"connections,omitempty"`
	CreatedAt   time.Time                  `bson:"created_at"`
	UpdatedAt   time.Time                  `bson:"updated_at"`
}

// TOTP secrets are stored encrypted; see auth.Service.
type TOTP struct {
	Enabled       bool   `bson:"enabled"`
	Secret        string `bson:"secret,omitempty"`
	PendingSecret string `bson:"pending_secret,omitempty"`
}

type Profile struct {
	Name      string `bson:"name,omitempty" json:"name"`
	Username  string `bson:"username,omitempty" json:"username"`
	DOB       string `bson:"dob,omitempty" json:"dob"`
	AvatarURL string `bson:"avatar_url,omitempty" json:"avatar_url,omitempty"`
	Bio       string `bson:"bio,omitempty" json:"bio,omitempty"`
}

func (p Profile) Complete() bool { return p.Name != "" && p.Username != "" && p.DOB != "" }

func (u *User) WebAuthnID() []byte                         { return u.WebAuthnUserID }
func (u *User) WebAuthnCredentials() []webauthn.Credential { return u.Passkeys }
func (u *User) WebAuthnDisplayName() string                { return u.WebAuthnName() }

func (u *User) WebAuthnName() string {
	switch {
	case u.Profile.Username != "":
		return u.Profile.Username
	case u.Email != "":
		return u.Email
	case u.Phone != "":
		return u.Phone
	}
	return u.ID.Hex()
}

type SignInMethods struct {
	Email      bool     `json:"email"`
	Truecaller bool     `json:"truecaller"`
	Passkeys   []string `json:"passkeys"`
}

type View struct {
	ID               string                     `json:"id"`
	Email            string                     `json:"email,omitempty"`
	Phone            string                     `json:"phone,omitempty"`
	Profile          Profile                    `json:"profile"`
	ProfileComplete  bool                       `json:"profile_complete"`
	TwoFactorEnabled bool                       `json:"two_factor_enabled"`
	SignInMethods    SignInMethods              `json:"sign_in_methods"`
	Connections      map[provider.Provider]bool `json:"connections"`
	CreatedAt        time.Time                  `json:"created_at"`
}

func PasskeyID(cred webauthn.Credential) string {
	return base64.RawURLEncoding.EncodeToString(cred.ID)
}

func (u *User) View() View {
	passkeys := make([]string, 0, len(u.Passkeys))
	for _, c := range u.Passkeys {
		passkeys = append(passkeys, PasskeyID(c))
	}
	return View{
		ID:               u.ID.Hex(),
		Email:            u.Email,
		Phone:            u.Phone,
		Profile:          u.Profile,
		ProfileComplete:  u.Profile.Complete(),
		TwoFactorEnabled: u.TOTP.Enabled,
		SignInMethods:    SignInMethods{Email: u.Email != "", Truecaller: u.TruecallerID != "", Passkeys: passkeys},
		Connections:      provider.Flags(u.Connections),
		CreatedAt:        u.CreatedAt,
	}
}
