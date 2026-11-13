package connection

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Provider string

const (
	YouTube      Provider = "youtube"
	Roblox       Provider = "roblox"
	Instagram    Provider = "instagram"
	X            Provider = "x"
	Spotify      Provider = "spotify"
	GitHub       Provider = "github"
	RiotGames    Provider = "riot_games"
	LinkedIn     Provider = "linkedin"
	Reddit       Provider = "reddit"
	PlayStation  Provider = "playstation"
	CustomDomain Provider = "custom_domain"
	Crunchyroll  Provider = "crunchyroll"
	AmazonPrime  Provider = "amazon_prime"
	Xbox         Provider = "xbox"
	PayPal       Provider = "paypal"
	Steam        Provider = "steam"
	EpicGames    Provider = "epic_games"
)

var Providers = []Provider{
	YouTube, Roblox, Instagram, X, Spotify, GitHub, RiotGames, LinkedIn, Reddit,
	PlayStation, CustomDomain, Crunchyroll, AmazonPrime, Xbox, PayPal, Steam, EpicGames,
}

func (p Provider) Valid() bool {
	for _, known := range Providers {
		if p == known {
			return true
		}
	}
	return false
}

type EventType string

const (
	EventConnected    EventType = "connection.connected"
	EventUpdated      EventType = "connection.updated"
	EventDisconnected EventType = "connection.disconnected"
)

// Account is the provider profile sent by the OAuth service. Tokens never reach this service.
type Account struct {
	ID          string         `bson:"id" json:"id"`
	Username    string         `bson:"username,omitempty" json:"username,omitempty"`
	DisplayName string         `bson:"display_name,omitempty" json:"display_name,omitempty"`
	Email       string         `bson:"email,omitempty" json:"email,omitempty"`
	AvatarURL   string         `bson:"avatar_url,omitempty" json:"avatar_url,omitempty"`
	ProfileURL  string         `bson:"profile_url,omitempty" json:"profile_url,omitempty"`
	Metadata    map[string]any `bson:"metadata,omitempty" json:"metadata,omitempty"`
}

type Connection struct {
	ID             bson.ObjectID `bson:"_id,omitempty"`
	UserID         bson.ObjectID `bson:"user_id"`
	Provider       Provider      `bson:"provider"`
	Connected      bool          `bson:"connected"`
	Account        *Account      `bson:"account,omitempty"`
	ConnectedAt    *time.Time    `bson:"connected_at,omitempty"`
	DisconnectedAt *time.Time    `bson:"disconnected_at,omitempty"`
	LastEventID    string        `bson:"last_event_id"`
	LastEventAt    time.Time     `bson:"last_event_at"`
	UpdatedAt      time.Time     `bson:"updated_at"`
}

type View struct {
	Provider    Provider   `json:"provider"`
	Connected   bool       `json:"connected"`
	Account     *Account   `json:"account,omitempty"`
	ConnectedAt *time.Time `json:"connected_at,omitempty"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

type Event struct {
	ID         string    `json:"id"`
	Type       EventType `json:"type"`
	OccurredAt time.Time `json:"occurred_at"`
	UserID     string    `json:"user_id"`
	Provider   Provider  `json:"provider"`
	Account    *Account  `json:"account,omitempty"`
}
