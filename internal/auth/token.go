package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type Tokens struct {
	secret []byte
	ttl    time.Duration
}

func NewTokens(secret string, ttl time.Duration) Tokens {
	return Tokens{secret: []byte(secret), ttl: ttl}
}

func (t Tokens) Issue(userID bson.ObjectID) (string, time.Time, error) {
	now := time.Now()
	expires := now.Add(t.ttl)
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   userID.Hex(),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(expires),
	}).SignedString(t.secret)
	return token, expires, err
}

func (t Tokens) Parse(raw string) (bson.ObjectID, error) {
	var claims jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(raw, &claims, func(*jwt.Token) (any, error) { return t.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired())
	if err != nil {
		return bson.ObjectID{}, err
	}
	id, err := bson.ObjectIDFromHex(claims.Subject)
	if err != nil {
		return bson.ObjectID{}, errors.New("invalid subject")
	}
	return id, nil
}
